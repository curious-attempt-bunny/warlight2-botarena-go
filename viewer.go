package main

import "log"
import "fmt"
import "net/http"
import "io/ioutil"
import "strings"

func Proxy(w http.ResponseWriter, r *http.Request) {
    fmt.Println("Proxying for "+r.URL.String())
    res, err := http.Get("http://theaigames.com"+r.URL.String())
    if err != nil {
        log.Fatal(err)
    }
    content, err := ioutil.ReadAll(res.Body)
    res.Body.Close()
    if err != nil {
        log.Fatal(err)
    }

    for key, values := range res.Header {
        fmt.Println("Passing through response header: "+key)
        w.Header()[key] = values
    }

    output := string(content)
    output = strings.Replace(output, "(MISSING)", "", -1) // TODO is this an encoding issue?

    // fmt.Println(output)

    fmt.Fprintf(w, output)

}

func main() {
    http.HandleFunc("/competitions/warlight-ai-challenge-2/games/", func(w http.ResponseWriter, r *http.Request) {
        content := "It's working!"
        fmt.Fprintln(w, content)
    })
    http.HandleFunc("/", Proxy)
    err := http.ListenAndServe(":8080", nil)
    if err != nil {
        log.Fatal(err)
    }
}