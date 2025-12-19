// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"crypto/tls"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	rv8 "github.com/go-redis/redis/v8"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/hooks/storage/redis"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
	tcpAddr := flag.String("tcp", ":1883", "network address for TCP listener")
	wsAddr := flag.String("ws", ":1882", "network address for Websocket listener")
	infoAddr := flag.String("info", ":8080", "network address for web info dashboard listener")
	tlsCertFile := flag.String("tls-cert-file", "", "TLS certificate file")
	tlsKeyFile := flag.String("tls-key-file", "", "TLS key file")
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	var tlsConfig *tls.Config

	if tlsCertFile != nil && tlsKeyFile != nil && *tlsCertFile != "" && *tlsKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(*tlsCertFile, *tlsKeyFile)
		if err != nil {
			return
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	server := mqtt.New(nil)
	// server := mqtt.New(&mqtt.Options{
	// 	Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	// 		Level: slog.LevelDebug,
	// 	})),
	// })
	_ = server.AddHook(new(auth.Hook), &auth.Options{
		Ledger: &auth.Ledger{
			Auth: []auth.AuthRule{
				{
					Username: "mqtt", // 用户名密码匹配
					Password: "aU8Zqus6gbjzo7mW",
					Allow:    true,
				},
			},
		},
	})
	// 先添加去重钩子，过滤重复消息
	deduplication := hooks.NewDeduplicationHook()
	server.AddHook(deduplication, nil)

	// 再添加 IP 注入钩子，只处理非重复消息
	ipInjector := hooks.NewIPInjectorHook()
	server.AddHook(ipInjector, nil)

	// connect := hooks.NewConnectHook(server)
	// server.AddHook(connect, nil)

	//构建时候会自己改地址跟密码
	err := server.AddHook(new(redis.Hook), &redis.Options{
		Options: &rv8.Options{
			Addr:     "192.168.0.147:6379", // Redis服务端地址
			Password: "W3gS3nslOOrRqRa6",   // Redis服务端的密码
			DB:       1,                    // Redis数据库的index
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	tcp := listeners.NewTCP(listeners.Config{
		ID:        "t1",
		Address:   *tcpAddr,
		TLSConfig: tlsConfig,
	})
	err = server.AddListener(tcp)
	if err != nil {
		log.Fatal(err)
	}

	ws := listeners.NewWebsocket(listeners.Config{
		ID:      "ws1",
		Address: *wsAddr,
	})
	err = server.AddListener(ws)
	if err != nil {
		log.Fatal(err)
	}

	stats := listeners.NewHTTPStats(
		listeners.Config{
			ID:      "info",
			Address: *infoAddr,
		},
		server.Info,
	)
	err = server.AddListener(stats)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	<-done
	server.Log.Warn("caught signal, stopping...")
	_ = server.Close()
	server.Log.Info("mochi mqtt shutdown complete")
}
