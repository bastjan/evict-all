package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	var ns, asUser, labelSel, nsLabelSel string
	var dryRun bool
	flag.StringVar(&ns, "namespace", "default", "namespace to evict pods from")
	flag.StringVar(&asUser, "as", "", "impersonate user")
	flag.StringVar(&labelSel, "l", "", "label selector to filter pods by")
	flag.StringVar(&nsLabelSel, "lns", "", "label selector to filter namespaces by, overrides the namespace flag")
	flag.BoolVar(&dryRun, "dry-run", false, "dry run")

	flag.Parse()

	cfg := config.GetConfigOrDie()
	cfg.Impersonate.UserName = asUser
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Println("failed to create client")
		os.Exit(1)
	}

	parsedSel, err := labels.Parse(labelSel)
	if err != nil {
		fmt.Println("failed to parse label selector")
		os.Exit(1)
	}

	var nsl corev1.NamespaceList
	if nsLabelSel != "" {
		parsedNsSel, err := labels.Parse(nsLabelSel)
		if err != nil {
			fmt.Println("failed to parse label selector")
			os.Exit(1)
		}
		if err := cl.List(context.Background(), &nsl, client.MatchingLabelsSelector{
			Selector: parsedNsSel,
		}); err != nil {
			fmt.Println("failed to list namespaces")
			os.Exit(1)
		}
	} else {
		nsl.Items = []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: ns}}}
	}

	var errs error
	for _, ns := range nsl.Items {
		errs = multierr.Append(errs, evict(cl, ns.Name, parsedSel, dryRun))
	}

	if errs != nil {
		fmt.Println("failed to evict pods")
		os.Exit(1)
	}
}

func evict(cl client.Client, ns string, sel labels.Selector, dryRun bool) error {
	var pl corev1.PodList
	if err := cl.List(context.Background(), &pl, client.InNamespace(ns), client.MatchingLabelsSelector{
		Selector: sel,
	}); err != nil {
		fmt.Println("failed to list pods")
		os.Exit(1)
	}

	var dryRunOpts []string
	var dryRunMessage string
	if dryRun {
		dryRunOpts = []string{"All"}
		dryRunMessage = " (dry run)"
	}

	fmt.Printf("Evicting %d pods from %q%s\n", len(pl.Items), ns, dryRunMessage)
	var errs error
	for _, p := range pl.Items {
		fmt.Printf("Evicting %s/%s%s\n", ns, p.Name, dryRunMessage)
		errs = multierr.Append(errs, cl.SubResource("eviction").Create(context.Background(), &p, &policyv1.Eviction{
			DeleteOptions: &metav1.DeleteOptions{
				DryRun: dryRunOpts,
			},
		}))
	}

	return errs
}
