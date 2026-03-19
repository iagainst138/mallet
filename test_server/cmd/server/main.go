package main

import (
	"flag"
	"log"
	"time"

	"tserver"
)

type serverFunc func(string, bool) error

func main() {
	var delay time.Duration
	httpListenAddr := "127.0.0.1:8080"
	grpcListenAddr := "127.0.0.1:8081"
	enableErrors := false

	flag.StringVar(&httpListenAddr, "l", httpListenAddr, "http address to listen on")
	flag.StringVar(&grpcListenAddr, "g", grpcListenAddr, "grpc address to listen on")
	flag.BoolVar(&enableErrors, "with-errors", enableErrors, "enable random errors")
	flag.DurationVar(&delay, "delay", delay, "add a delay")
	flag.Parse()

	if enableErrors {
		log.Println("servers will return errors")
	}

	c := make(chan error)

	//go func() { c <- tserver.GrpcServer(grpcListenAddr, enableErrors, delay) }()
	go func() { c <- tserver.HttpServer(httpListenAddr, enableErrors, delay) }()

	err := <-c
	if err != nil {
		panic(err)
	}
}
