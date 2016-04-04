//see LICENSE
package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/dc0d/cx"
	"github.com/dimfeld/httptreemux"
)

func main() {
	app := cli.NewApp()
	app.Name = "dirsh"
	app.Usage = "share files in current directory (on http via a specific port)"
	app.Version = "0.1.1"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:   "port, p",
			Value:  9099,
			Usage:  "port to serve http on",
			EnvVar: "DIRSH_PORT",
		},
		cli.BoolFlag{
			Name:   "preview, w",
			Usage:  "turns preview on",
			EnvVar: "DIRSH_PREVIEW",
		},
	}
	app.Action = run
	app.Run(os.Args)

	workerRegistry.Wait()
}

func run(c *cli.Context) {
	workerRegistry.Add(1)

	port := c.Int("port")
	preview := c.Bool("preview")
	go serve(port, preview)
}

func serve(port int, preview bool) {
	defer workerRegistry.Done()

	dir, err := os.Getwd()
	if err != nil {
		log.Error(err)
	}

	mux := httptreemux.New()

	mux.GET("/dir/*path", func(res http.ResponseWriter, req *http.Request, params map[string]string) {
		cx.Plumb(nil, reqLogger(), serveContent(`/dir`, dir)).ServeHTTP(res, req)
	})

	mux.GET("/", func(res http.ResponseWriter, req *http.Request, params map[string]string) {
		cx.Plumb(nil, reqLogger(), listFiles(dir, preview)).ServeHTTP(res, req)
	})

	log.Info(`started to serve dir `, dir, ` on url http://ocalhost:`, port)

	strPort := fmt.Sprintf(":%v", port)
	http.ListenAndServe(strPort, mux)
}

func listFiles(searchDir string, preview bool) cx.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		var fh http.HandlerFunc = func(res http.ResponseWriter, req *http.Request) {
			fileList := []string{}
			err := filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
				if f.IsDir() {
					return nil
				}

				fileList = append(fileList, path)
				return nil
			})
			if err != nil {
				log.Error(err)
			}

			res.Write([]byte(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Shared Content</title>
  
  <style>
body {
    cursor: default;
    font-size: 14px;
    line-height: 21px;
    font-family: "Segoe UI","Helvetica",Garuda,Arial,sans-serif;
    padding: 18px 18px 18px 18px;
}
ul {
    margin-bottom: 14px;
    list-style: none;
}
li { margin: 0 0 7px 0; }
li a { 
    display: block;
    height: 30px;
    margin: 0 0 7px 0;
    background: #F7F5F2 url(data:image/gif;base64,iVBORw0KGgoAAAANSUhEUgAAABsAAAAcCAYAAACQ0cTtAAAAAXNSR0IArs4c6QAAAARnQU1BAACxjwv8YQUAAAAgY0hSTQAAeiYAAICEAAD6AAAAgOgAAHUwAADqYAAAOpgAABdwnLpRPAAAAcVJREFUSEvtVtFNxDAM7QiMwAiMwAiMwAhs4PZSic8bgRFuhI5wI3QERjjyYjuJrnaagoSExElRT4njZzvPLxmG/99vVIBofhrHcB6neYnjM46bfBfME4XnH8cBJwIA5zoAqKOaD+u3QSUTdjaGD6LTixU95rGeg4mZHsqybO6PNpV6ClcNrguQpjBylOFKRA9dm8QI9hVgO0OOjg/fA6LT/IqA8LXLWgCbZ6hkaBlVhFm8rIVYCNq2IXp/lKxcJ3DeAyZ2F/hDtTZB0TS/pUWnPLqhFyyxFP5iyTdgykBk2CKFBeadr1spdWIB1cy8B1MGOvtW89zaYKUVarCa6jYYK822jCxBN2eTyBKaVhqX7RMJGvugowYYC63JHpCm0sZaJ9N/i1ScdQrksmWjOAQrzWZ1AE22RQcaoOmvRBJWt1mzlMkNEAXYsy0C4bA709/JLjWrKnwDSHsMtm4bSXY41KiNRuer4DYaX/RVfLR7dshR7QDaIpyEPN3ke0qU99fs8whwDyZydwxInUiG8t4Iq/XWgLozSFzXq2lHW5tnKJepgOZ3yKbX+Omwc0Yt0a3XkKkAQzWSDKVLFPMHb/RezL9l9wWuQAy9JbrovAAAAABJRU5ErkJggg==) 97% center no-repeat;
    font-size: 18px;
    color: #333;
    padding: 5px 0 0 20px;
    text-decoration: none;
}

li a:hover { background-color: #EFEFEF; }

.orange { border-left: 5px solid #F5876E; }

.blue{ border-left: 5px solid #61A8DC; }

.green{ border-left: 5px solid #8EBD40; }

.purple { border-left: 5px solid #988CC3; }

.gold { border-left: 5px solid #D8C86E; }

  </style>  
  
</head>

<body>`))

			res.Write([]byte("<ul>"))

			classes := []string{"orange", "blue", "green", "purple", "gold"}
			liTemplate := `
				<li class="%s">
				    <a href='%s' download='%s'>%s</a>
				    <video width="100%%" height="100%%" controls>
				        <source src="%s" %s>
				        Your browser does not support the video tag.
				    </video>
				</li>`

			for _, f := range fileList {
				colorIndex := rand.Intn(5)
				liClass := classes[colorIndex]

				s, _ := filepath.Rel(searchDir, f)
				_, fn := filepath.Split(f)
				ext := strings.ToLower(filepath.Ext(fn))
				src := fmt.Sprintf("/dir/%s", s)

				if !preview {
					res.Write([]byte(fmt.Sprintf(`<li class="%s">
    <a href='/dir/%s' download='%s'>%s</a> 
</li>`, liClass, s, fn, fn)))
				} else {

					switch ext {
					case ".3gpp":
						res.Write([]byte(fmt.Sprintf(liTemplate, liClass, src, fn, fn, src, `type="video/3gpp"`)))
					case ".ogv":
						res.Write([]byte(fmt.Sprintf(liTemplate, liClass, src, fn, fn, src, `type="video/ogg"`)))
					case ".webm":
						res.Write([]byte(fmt.Sprintf(liTemplate, liClass, src, fn, fn, src, `type="video/webm"`)))
					case ".mp4":
						res.Write([]byte(fmt.Sprintf(liTemplate, liClass, src, fn, fn, src, `type="video/mp4"`)))
					case ".mkv":
						res.Write([]byte(fmt.Sprintf(liTemplate, liClass, src, fn, fn, src, ``)))
					default:
						res.Write([]byte(fmt.Sprintf(`<li class="%s">
    <a href='/dir/%s' download='%s'>%s</a> 
</li>`, liClass, s, fn, fn)))
					}
				}
			}
			res.Write([]byte("</ul>"))

			res.Write([]byte(`</body>
             
</html>`))
		}

		return fh
	}
}

func serveContent(prefix, dir string) cx.ContextProvider {
	return func(ctx cx.Context) cx.MiddlewareFunc {
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

func reqLogger() cx.MiddlewareFunc {
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

func init() {
	log.SetFormatter(&log.TextFormatter{
		ForceColors: true,
	})
}

var (
	workerRegistry = new(sync.WaitGroup)
)

const (
	//XRealIP +
	XRealIP = "X-Real-IP"

	//XForwardedFor +
	XForwardedFor = "X-Forwarded-For"
)
