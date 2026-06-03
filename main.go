// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"

	"strconv"
	"strings"
	"time"

	"sdv.googlesource.com/mcg/mcg"
	"sdv.googlesource.com/mcg/mcg/docs"
	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/vs_cache"

	"github.com/gin-gonic/gin"
)

var (
	listenString   = flag.String("listen", ":8005", "tcp listen string for http server")
	publicURL      = flag.String("pub", "/", "The base URL of this server, which is used for OpenAPI docs. Defaults to `/`, which maps to whatever base URL a user accesses the docs from.")
	trustedProxies = flag.String("trustedProxies", "", "comma-separated list of trusted proxy IPs (v4, v4/CIDR, v6, v6/CIDR)")
)

func main() {
	flag.Parse()
	ctx := context.Background()

	cConfig, err := readCacheConfigFromEnv()
	if err != nil {
		log.Fatalf("[Cache] Failed to configure cache: %v", err)
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, cConfig)
	if err != nil {
		log.Fatalf("[Cache] Failed to create cache: %v", err)
	}
	defer closeCache()

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(vs_cache.CacheMiddleware(cache))
	r.Use(mcgerrors.MiddlewareRenderErrors)
	r.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		// MiddlewareRenderErrors above will render the panic
		c.Error(mcgerrors.InternalFromPanic(err))
	}))

	r.SetTrustedProxies(strings.Split(*trustedProxies, ","))

	mcgServer := &mcg.Server{}
	mcgServer.InstallRoutes(r)

	if err := docs.InstallRoutes(*publicURL, r); err != nil {
		log.Fatalf("Failed to initialize OpenAPI docs: %v", err)
	}

	// TODO: b/382625305 - Remove this redirect. It is only here to redirect to
	// the new location of the Swagger UI while people get adjusted to it.
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusPermanentRedirect, "/docs/swagger-ui")
	})

	// Start server, listen for interrupts, and gracefully flush metrics
	var s *http.Server = &http.Server{}
	s.Handler = r.Handler()
	s.Addr = *listenString

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt)
	go func() {
		err := s.ListenAndServe()
		log.Println(err)
		shutdownCh <- os.Kill
	}()

	fmt.Fprintln(os.Stderr, "shutting down due to signal",
		<-shutdownCh,
	)
	shutdownCtx, cancel := context.WithTimeout(ctx, time.Millisecond*3000)
	defer cancel()
	// Give HTTP server 1 second less than overall shutdown
	httpShutdownCtx, cancel := context.WithTimeout(shutdownCtx, time.Millisecond*2000)
	defer cancel()
	if err := s.Shutdown(httpShutdownCtx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}
}

func readCacheConfigFromEnv() (vs_cache.CacheConfig, error) {
	var cConfig vs_cache.CacheConfig

	if v, ok := os.LookupEnv("MCG_LOCALCACHE"); ok && slices.Contains([]string{"true", "1", "yes", "y"}, strings.ToLower(v)) {
		cConfig.EnableLocalCache = true
		log.Println("[Cache] Local Cache enabled")
	}

	if v, ok := os.LookupEnv("MCG_LOCALCACHE_CAP"); ok {
		cap, err := strconv.Atoi(v)
		if err != nil {
			return cConfig, err
		}
		cConfig.LocalCapacity = cap
		log.Println("[Cache] Local Cache Size: ", cConfig.LocalCapacity)
	}

	if rCacheAddr, ok := os.LookupEnv("MCG_REDISCACHE_HOSTS"); ok {
		cConfig.EnableRedCache = true

		addresses := strings.Split(rCacheAddr, ";")
		if len(addresses) == 1 && addresses[0] == "" {
			// MCG_REDISCACHE_HOSTS was empty, do nothing.
			return cConfig, nil
		}
		log.Printf("[Cache] Redis endpoint(s) provided: %s\n", rCacheAddr)

		var rCachePass string
		if pass, ok := os.LookupEnv("MCG_REDISCACHE_PASS"); ok {
			rCachePass = pass
		}

		// Default to cluster mode if multiple hosts are provided.
		isCluster := len(addresses) > 1
		// Allow overriding the cluster mode detection with an explicit environment variable.
		if v, ok := os.LookupEnv("MCG_REDISCACHE_CLUSTER"); ok {
			var err error
			isCluster, err = strconv.ParseBool(v)
			if err != nil {
				return cConfig, fmt.Errorf("invalid boolean value for MCG_REDISCACHE_CLUSTER: %q", v)
			}
			log.Printf("[Cache] Redis cluster mode explicitly set to %v by MCG_REDISCACHE_CLUSTER.", isCluster)
		}

		if isCluster {
			cConfig.IsRedisCluster = true
			cConfig.RedClusterOpts.Addrs = addresses
			cConfig.RedClusterOpts.Password = rCachePass
			log.Println("[Cache] Configuring for Redis Cluster mode.")
		} else { // Standalone mode
			if len(addresses) > 1 {
				return cConfig, fmt.Errorf("multiple Redis hosts provided but cluster mode is disabled. For standalone mode, provide only one host in MCG_REDISCACHE_HOSTS")
			}
			cConfig.IsRedisCluster = false
			cConfig.RedOpts.Addr = addresses[0]
			cConfig.RedOpts.Password = rCachePass
			log.Println("[Cache] Configuring for standalone Redis mode.")
		}
	}

	return cConfig, nil
}
