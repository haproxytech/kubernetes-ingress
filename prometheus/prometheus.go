package prometheus

import (
	"fmt"
	"log"
	"net/http"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Start(args utils.OSArgs) {
	if args.PromotheusPort != 0 {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", args.PromotheusPort), nil))
	}
}
