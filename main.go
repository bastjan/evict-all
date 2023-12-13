package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	ns := flag.String("namespace", "default", "namespace to evict pods from")
	asUser := flag.String("as", "", "impersonate user")

	flag.Parse()

	cfg := config.GetConfigOrDie()
	cfg.Impersonate.UserName = *asUser
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Println("failed to create client")
		os.Exit(1)
	}

	var pl corev1.PodList
	if err := cl.List(context.Background(), &pl, client.InNamespace(*ns)); err != nil {
		fmt.Println("failed to list pods")
		os.Exit(1)
	}

	fmt.Printf("evicting %d pods\n", len(pl.Items))
	var errs error
	for _, p := range pl.Items {
		fmt.Println(p.Name)
		errs = multierr.Append(errs, cl.SubResource("eviction").Create(context.Background(), &p, &policyv1.Eviction{}))
	}

	if errs != nil {
		fmt.Println("failed to evict pods")
		os.Exit(1)
	}
}
