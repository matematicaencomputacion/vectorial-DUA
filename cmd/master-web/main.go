package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/pkg/webgateway"
)

//go:embed web
var webFS embed.FS

const (
	defaultWebAddr    = ":8080"
	defaultRouterAddr = "127.0.0.1:50051"
)

func main() {
	webAddr := defaultWebAddr
	if v := os.Getenv("AVLP_WEB_ADDR"); v != "" {
		webAddr = v
	}
	routerAddr := defaultRouterAddr
	if v := os.Getenv("AVLP_ROUTER_ADDR"); v != "" {
		routerAddr = normalizeDialAddr(v)
	}

	conn, err := grpc.NewClient(routerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial router %s: %v", routerAddr, err)
	}
	defer conn.Close()

	staticRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("embed web: %v", err)
	}

	gw := webgateway.New(vectorv1.NewVectorRouterClient(conn), http.FileServer(http.FS(staticRoot)))
	srv := &http.Server{
		Addr:              webAddr,
		Handler:           gw.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("master-web listening on http://%s (router=%s)", webAddr, routerAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// normalizeDialAddr maps listen-style ":50051" to a loopback dial target.
func normalizeDialAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}
