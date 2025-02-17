// Copyright Contributors to the Open Cluster Management project
package accept

import (
	"fmt"
	"os"

	genericclioptionsclusteradm "open-cluster-management.io/clusteradm/pkg/genericclioptions"
	"open-cluster-management.io/clusteradm/pkg/helpers"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var example = `
# Accept clusters
%[1]s accept --clusters <cluster_1>,<cluster_2>,...
# Accept clusters in foreground
%[1]s accept --clusters <cluster_1>,<cluster_2>,... --wait
`

// NewCmd ...
func NewCmd(clusteradmFlags *genericclioptionsclusteradm.ClusteradmFlags, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewOptions(clusteradmFlags, streams)

	cmd := &cobra.Command{
		Use:   "accept",
		Short: "accept a list of clusters",
		Long: "accept the join request from managed cluster - the CSR from your managed cluster will be approved, " +
			"and additionally it will prescribe the OCM hub control plane to setup related resources",
		Example:      fmt.Sprintf(example, helpers.GetExampleHeader()),
		SilenceUsage: true,
		PreRun: func(c *cobra.Command, args []string) {
			helpers.DryRunMessage(o.ClusteradmFlags.DryRun)
		},
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.complete(c, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&o.Clusters, "clusters", "", "Names of the cluster to accept (comma separated)")
	cmd.Flags().BoolVar(&o.Wait, "wait", false, "If set, wait for the managedcluster and CSR in foreground.")
	cmd.Flags().BoolVar(&o.SkipApproveCheck, "skip-approve-check", false, "If set, then skip check and approve csr directly.")

	cmd.Flags().StringVar(&o.awsHubEksClusterName, "aws-hub-eks-cluster", "", "AWS only: the name of the hub EKS cluster")
	cmd.Flags().StringToStringVar(&o.awsAdditionalTags, "aws-tags", nil, "AWS only: additional tags to add to any created resources")
	cmd.Flags().StringVar(&o.awsRegion, "aws-region", os.Getenv("AWS_REGION"), "AWS only: The region the worker cluster resides in")

	return cmd
}
