package main

import "log"
import "fmt"
import "net/http"
// import "io/ioutil"
import "strings"
import "io"
import "os"

func Proxy(w http.ResponseWriter, r *http.Request) {
    fmt.Println("Proxying for "+r.URL.String())
    res, err := http.Get("http://theaigames.com"+r.URL.String())
    if err != nil {
        log.Fatal(err)
    }

    for key, values := range res.Header {
        // fmt.Println("Passing through response header: "+key)
        w.Header()[key] = values
    }

    io.Copy(w, res.Body)
    res.Body.Close()
}

func main() {
    http.HandleFunc("/competitions/warlight-ai-challenge-2/games/", func(w http.ResponseWriter, r *http.Request) {
        if strings.Index(r.URL.String(), "/data") >= 0 {
            data, err := os.Open("game-data.txt")
            if err != nil {
                log.Fatal(err)
            }
            io.Copy(w, data)
        } else {
            Proxy(w, r)
        }
    })
    http.HandleFunc("/", Proxy)
    err := http.ListenAndServe(":80", nil)
    if err != nil {
        log.Fatal(err)
    }
}