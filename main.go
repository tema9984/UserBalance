package main

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tema9984/AvitoInter/config"
	balance "github.com/tema9984/AvitoInter/internal/balance"
)

const configPath = "config/config.json"

func main() {
	cfg, err := config.ParseConfig(configPath)
	if err != nil {
		logrus.Fatal(err)
	}
	mux := http.NewServeMux()
	svc := balance.NewService(cfg)
	svc.ConfigureService()
	mux.HandleFunc("/transaction", svc.HandlerTransaction)
	mux.HandleFunc("/balance", svc.HandlerGetBalance)
	mux.HandleFunc("/replenish", svc.HandlerReplenish)
	mux.HandleFunc("/history", svc.HandlerHistory)
	hand := svc.Logging(mux)
	cors := cors(hand)

	s := &http.Server{
		Addr:         ":" + cfg.WebPort,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      cors,
	}
	logrus.Info("server started")
	logrus.Fatal(s.ListenAndServe())
}
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Accept")
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS,POST,GET")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, req)
		w.WriteHeader(http.StatusOK)
	})
}
