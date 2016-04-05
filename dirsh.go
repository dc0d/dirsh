//see LICENSE
package main

import (
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
	app.Action = func(c *cli.Context) {
		workerRegistry.Add(1)

		port := c.Int("port")
		preview := c.Bool("preview")
		go serve(port, preview)
	}
	app.Run(os.Args)

	workerRegistry.Wait()
}

func serve(port int, preview bool) {
	defer workerRegistry.Done()

	dir, err := os.Getwd()
	if err != nil {
		log.Error(err)
	}

	mux := httptreemux.New()

	mux.GET("/dir/*path", func(res http.ResponseWriter, req *http.Request, params map[string]string) {
		cx.Plumb(nil, reqLogger(), recoverPlumbing(), serveContent(`/dir`, dir)).ServeHTTP(res, req)
	})

	mux.GET("/", func(res http.ResponseWriter, req *http.Request, params map[string]string) {
		cx.Plumb(nil, reqLogger(), recoverPlumbing(), listFiles(dir, preview)).ServeHTTP(res, req)
	})

	mux.GET("/preview/:mediatype/*path", func(res http.ResponseWriter, req *http.Request, params map[string]string) {
		mediaType := params["mediatype"]

		src := req.URL.String()
		r2 := prefixRegexp.FindAllStringSubmatch(src, -1)[0]
		n1 := prefixRegexp.SubexpNames()

		md := map[string]string{}
		for i, n := range r2 {
			md[n1[i]] = n
		}

		urlRest, ok := md["url_rest"]
		if ok {
			src = urlRest
		}

		cx.Plumb(nil, reqLogger(), recoverPlumbing(), playMedia(mediaType, src)).ServeHTTP(res, req)
	})

	log.Info(`started to serve dir `, dir, ` on url http://ocalhost:`, port)

	strPort := fmt.Sprintf(":%v", port)
	http.ListenAndServe(strPort, mux)
}

var (
	prefixRegexp = regexp.MustCompile(`/[^/]+/[^/]+(?P<url_rest>.*)`)
)

func playMedia(mediaType, src string) cx.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		var fh http.HandlerFunc = func(res http.ResponseWriter, req *http.Request) {
			if mediaType == "none" {
				mediaType = ``
			}

			data := struct {
				Src, MediaType string
			}{
				Src:       src,
				MediaType: mediaType,
			}

			pt := template.New("PlayerTemplate")
			t, err := pt.Parse(playerTemplate)
			if err != nil {
				log.Error(err)
			}
			err = t.Execute(res, data)
			if err != nil {
				log.Error(err)
			}

			// this is a final action
			// next.ServeHTTP(res, req)
		}

		return fh
	}
}

func listFiles(searchDir string, preview bool) cx.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		var fh http.HandlerFunc = func(res http.ResponseWriter, req *http.Request) {
			data := fileItems(searchDir)
			pt := template.New("FileListTemplate")
			t, err := pt.Parse(fileListTemplate)
			if err != nil {
				log.Error(err)
			}
			err = t.Execute(res, data)
			if err != nil {
				log.Error(err)
			}

			// this is a final action
			// next.ServeHTTP(res, req)
		}

		return fh
	}
}

func fileItems(searchDir string) []*FileItem {
	fileList := listFilesInside(searchDir)
	classes := []string{"orange", "blue", "green", "purple", "gold"}

	var data []*FileItem
	for _, f := range fileList {
		colorIndex := rand.Intn(5)
		liClass := classes[colorIndex]

		s, _ := filepath.Rel(searchDir, f)
		_, fn := filepath.Split(f)
		ext := strings.ToLower(filepath.Ext(fn))
		src := fmt.Sprintf("/dir/%s", s)

		item := &FileItem{s, fn, liClass, ext, src, decideType(ext)}
		data = append(data, item)
	}

	return data
}

func decideType(ext string) (res string) {
	switch ext {
	case ".3gpp":
		res = "video/3gpp"
	case ".ogv":
		res = "video/ogg"
	case ".webm":
		res = "video/webm"
	case ".mp4":
		res = "video/mp4"
	// case ".mkv":
	default:
		res = `none`
	}

	res = strings.Replace(res, "/", "%2f", -1)

	return res
}

//FileItem +
type FileItem struct {
	Path, Name, Class, Ext, Src, MediaType string
}

const (
	fileListTemplate = `<!doctype html>
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

<body>
    <ul>
    {{range .}}
        <!-- Path, Name, Class, Ext, Src, Type  -->
        <li class="{{.Class}}">
            <a href='{{.Src}}' download='{{.Name}}'>{{.Name}}</a>
            <a href='/preview/{{.MediaType}}{{.Src}}' target="_blank">+</a>
        </li>
    {{end}}
    </ul>
</body>
             
</html>`

	playerTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Shared Content</title>
</head>

<body>
    <video width="100%%" height="100%%" controls>
        <source src="{{.Src}}" type="{{.MediaType}}">
        Your browser does not support the video tag.
    </video>
</body>
             
</html>`
)
