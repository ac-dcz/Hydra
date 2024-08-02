package main

import (
	"WuKong/config"
	"WuKong/logger"
	"WuKong/node"

	"github.com/spf13/cobra"
)

var (
	loglevel, nodes, N, T, nodeID                                                int
	path, keyFile, tssKeyFile, committeeFile, parametersFile, storePath, logPath string
)

func main() {
	var rootCmd = cobra.Command{
		Use:   "node",
		Short: "A research implementation of the BFT Consensus protocol.",
	}
	//SubCommand

	var keyCmd = cobra.Command{
		Use:   "keys",
		Short: "Print a fresh key pair to file",
	}
	keyCmd.Flags().StringVar(&path, "path", "", "The file where to print the new key pair")
	keyCmd.Flags().IntVar(&nodes, "nodes", 4, "The number of new key pair")

	var tssKeyCmd = cobra.Command{
		Use:   "threshold_keys",
		Short: "Print fresh threshold key pairs to files",
	}
	tssKeyCmd.Flags().StringVar(&path, "path", "", "The file where to print the new tss key pair")
	tssKeyCmd.Flags().IntVar(&N, "N", 4, "N")
	tssKeyCmd.Flags().IntVar(&T, "T", 3, "T")

	var runCmd = cobra.Command{
		Use:   "run",
		Short: "Runs a single node",
	}

	runCmd.Flags().StringVar(&keyFile, "keys", "", "The file containing the node keys")
	runCmd.Flags().StringVar(&tssKeyFile, "threshold_keys", "", "The file containing the node threshold_keys")
	runCmd.Flags().StringVar(&committeeFile, "committee", "", "The file containing committee information")
	runCmd.Flags().StringVar(&parametersFile, "parameters", "", "The file containing the node parameter")
	runCmd.Flags().StringVar(&storePath, "store", "", "The path where to create the data store")
	runCmd.Flags().StringVar(&logPath, "log_out", "", "Teh path where to write log")
	runCmd.Flags().IntVar(&loglevel, "log_level", int(logger.DeployLevel), "The level of log out")
	runCmd.Flags().IntVar(&nodeID, "node_id", 0, "The ID of node")

	var deployCmd = cobra.Command{
		Use:   "deploy",
		Short: "Deploys a network of nodes locally",
	}

	deployCmd.Flags().IntVar(&nodes, "nodes", 4, "The number of nodes to deploy")

	rootCmd.AddCommand(&keyCmd, &tssKeyCmd, &runCmd, &deployCmd)

	//Generate key pair
	keyCmd.Run = func(cmd *cobra.Command, args []string) {
		config.GenerateKeyFiles(nodes, path)
	}

	//Generate tss key pair
	tssKeyCmd.Run = func(cmd *cobra.Command, args []string) {
		config.GenerateTsKeyFiles(N, T, path)
	}

	//run
	runCmd.Run = func(cmd *cobra.Command, args []string) {
		if node, err := node.NewNode(
			keyFile,
			tssKeyFile,
			committeeFile,
			parametersFile,
			storePath,
			logPath,
			loglevel,
			nodeID,
		); err != nil {
			logger.Error.Println(err)
			panic(err)
		} else {
			//blocking
			node.AnalyzeBlock()
		}
	}

	if err := runCmd.Execute(); err != nil {
		logger.Error.Println(err)
		panic(err)
	}

}
