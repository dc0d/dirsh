//see LICENSE
package main

import (
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/dc0d/plumber"
)

func serveContent(prefix, dir string) plumber.ContextProvider {
	return func(ctx interface{}) plumber.MiddlewareFunc {
		return func(next http.Handler) http.Handler {
			var fh http.HandlerFunc = func(res http.ResponseWriter, req *http.Request) {
				nmd := http.Dir(dir)
				fsrv := http.FileServer(nmd)
				http.StripPrefix(prefix, fsrv).ServeHTTP(res, req)

				next.ServeHTTP(res, req)
			}

			return fh
		}
	}
}

func recoverPlumbing() plumber.ContextProvider {
	return func(ctx interface{}) plumber.MiddlewareFunc {
		return func(next http.Handler) http.Handler {
			var fh http.HandlerFunc = func(res http.ResponseWriter, req *http.Request) {
				defer func() {
					if err := recover(); err != nil {
						trace := make([]byte, 1<<16)
						n := runtime.Stack(trace, true)
						errMsg := fmt.Errorf("panic recover\n %v\n stack trace %d bytes\n %s",
							err, n, trace[:n])
						log.Error(errMsg)
					}
				}()

				next.ServeHTTP(res, req)
			}

			return fh
		}
	}
}

func reqLogger() plumber.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		var fh http.HandlerFunc = func(res http.ResponseWriter, req *http.Request) {
			remoteAddr := req.RemoteAddr
			if ip := req.Header.Get(XRealIP); ip != "" {
				remoteAddr = ip
			} else if ip = req.Header.Get(XForwardedFor); ip != "" {
				remoteAddr = ip
			} else {
				remoteAddr, _, _ = net.SplitHostPort(remoteAddr)
			}

			hasError := false

			start := time.Now()

			next.ServeHTTP(res, req)

			stop := time.Now()
			method := req.Method
			path := req.URL.Path
			if path == "" {
				path = "/"
			}

			logMsg := fmt.Sprintf("%v %v %v %v", remoteAddr, method, path, stop.Sub(start))
			if hasError {
				log.Warn(logMsg)
			} else {
				log.Info(logMsg)
			}

		}
		return fh
	}
}
