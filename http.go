package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"go/build"
	"github.com/broady/mtgprice/mtgprice"
)

var client *mtgprice.Client

func main() {
	staticDir, err := staticDir()
	if err != nil {
		log.Fatalf("could not find static dir")
	}
	c, err := mtgprice.Open(mtgprice.Opts{
		Filename: "mtgprice.db",
		CardData: filepath.Join(staticDir, "AllSets.json"),
	})
	if err != nil {
		log.Fatalf("could not open db: %v", err)
	}
	client = c

	handleInterrupt()

	http.Handle("/", serveFile(filepath.Join(staticDir, "index.html")))
	http.Handle("/static", http.StripPrefix("static", http.FileServer(http.Dir(staticDir))))
	http.HandleFunc("/api/price", priceHandler)
	http.HandleFunc("/api/info", infoHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func staticDir() (string, error) {
	if fi, err := os.Stat("static"); err == nil && fi.IsDir() {
		return "static", nil
	}
	pkg, err := build.Import("github.com/broady/mtgprice", "", build.FindOnly)
	if err != nil {
		return "", err
	}
	return filepath.Join(pkg.Dir, "static"), nil
}

func priceHandler(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("cardName")
	_, ok := client.CardInfo(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, err := client.PriceForCard(name)
	if err != nil {
		log.Printf("card err: %v", err)
		http.NotFound(w, r)
		return
	}
	json.NewEncoder(w).Encode(p)
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("cardName")
	i, err := client.RichInfo(name)
	if err != nil {
		log.Printf("card err (%s): %v", name, err)
		http.NotFound(w, r)
		return
	}
	json.NewEncoder(w).Encode(i)
}

func handleInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Printf("shutting down...")
		if err := client.Close(); err != nil {
			log.Fatalf("could not clean up: %v", err)
		}
		os.Exit(1)
	}()
}

func serveFile(fn string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, fn)
	})
}
