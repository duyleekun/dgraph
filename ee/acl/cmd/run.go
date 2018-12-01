// +build !oss

/*
 * Copyright 2018 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Dgraph Community License (the "License"); you
 * may not use this file except in compliance with the License. You
 * may obtain a copy of the License at
 *
 *     https://github.com/dgraph-io/dgraph/blob/master/licenses/DCL.txt
 */

package acl

import (
	"os"
	"strings"

	"github.com/dgraph-io/dgo"
	"github.com/dgraph-io/dgo/protos/api"
	"github.com/dgraph-io/dgraph/x"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type options struct {
	dgraph string
}

var opt options
var tlsConf x.TLSHelperConfig

var CmdAcl x.SubCommand

func init() {
	CmdAcl.Cmd = &cobra.Command{
		Use:   "acl",
		Short: "Run the Dgraph acl tool",
	}

	flag := CmdAcl.Cmd.PersistentFlags()
	flag.StringP("dgraph", "d", "127.0.0.1:9080", "Dgraph gRPC server address")

	// TLS configuration
	x.RegisterTLSFlags(flag)
	flag.String("tls_server_name", "", "Used to verify the server hostname.")

	subcommands := initSubcommands()
	for _, sc := range subcommands {
		CmdAcl.Cmd.AddCommand(sc.Cmd)
		sc.Conf = viper.New()
		sc.Conf.BindPFlags(sc.Cmd.Flags())
		sc.Conf.BindPFlags(CmdAcl.Cmd.PersistentFlags())
		sc.Conf.SetEnvPrefix(sc.EnvPrefix)
	}
}

func initSubcommands() []*x.SubCommand {
	// user creation command
	var cmdUserAdd x.SubCommand
	cmdUserAdd.Cmd = &cobra.Command{
		Use:   "useradd",
		Short: "Run Dgraph acl tool to add a user",
		Run: func(cmd *cobra.Command, args []string) {
			if err := userAdd(cmdUserAdd.Conf); err != nil {
				glog.Errorf("Unable to add user:%v", err)
				os.Exit(1)
			}
		},
	}
	userAddFlags := cmdUserAdd.Cmd.Flags()
	userAddFlags.StringP("user", "u", "", "The user id to be created")
	userAddFlags.StringP("password", "p", "", "The password for the user")

	// user deletion command
	var cmdUserDel x.SubCommand
	cmdUserDel.Cmd = &cobra.Command{
		Use:   "userdel",
		Short: "Run Dgraph acl tool to delete a user",
		Run: func(cmd *cobra.Command, args []string) {
			if err := userDel(cmdUserDel.Conf); err != nil {
				glog.Errorf("Unable to delete the user:%v", err)
				os.Exit(1)
			}
		},
	}
	userDelFlags := cmdUserDel.Cmd.Flags()
	userDelFlags.StringP("user", "u", "", "The user id to be deleted")

	// login command
	var cmdLogIn x.SubCommand
	cmdLogIn.Cmd = &cobra.Command{
		Use:   "login",
		Short: "Login to dgraph in order to get a jwt token",
		Run: func(cmd *cobra.Command, args []string) {
			if err := userLogin(cmdLogIn.Conf); err != nil {
				glog.Errorf("Unable to login:%v", err)
				os.Exit(1)
			}
		},
	}
	loginFlags := cmdLogIn.Cmd.Flags()
	loginFlags.StringP("user", "u", "", "The user id to be created")
	loginFlags.StringP("password", "p", "", "The password for the user")

	// group creation command
	var cmdGroupAdd x.SubCommand
	cmdGroupAdd.Cmd = &cobra.Command{
		Use:   "groupadd",
		Short: "Run Dgraph acl tool to add a group",
		Run: func(cmd *cobra.Command, args []string) {
			if err := groupAdd(cmdGroupAdd.Conf); err != nil {
				glog.Errorf("Unable to add group:%v", err)
				os.Exit(1)
			}
		},
	}
	groupAddFlags := cmdGroupAdd.Cmd.Flags()
	groupAddFlags.StringP("group", "g", "", "The group id to be created")

	// group deletion command
	var cmdGroupDel x.SubCommand
	cmdGroupDel.Cmd = &cobra.Command{
		Use:   "groupdel",
		Short: "Run Dgraph acl tool to delete a group",
		Run: func(cmd *cobra.Command, args []string) {
			if err := groupDel(cmdGroupDel.Conf); err != nil {
				glog.Errorf("Unable to delete group:%v", err)
				os.Exit(1)
			}
		},
	}
	groupDelFlags := cmdGroupDel.Cmd.Flags()
	groupDelFlags.StringP("group", "g", "", "The group id to be deleted")

	// the usermod command used to set a user's groups
	var cmdUserMod x.SubCommand
	cmdUserMod.Cmd = &cobra.Command{
		Use:   "usermod",
		Short: "Run Dgraph acl tool to change a user's groups",
		Run: func(cmd *cobra.Command, args []string) {
			if err := userMod(cmdUserMod.Conf); err != nil {
				glog.Errorf("Unable to modify user:%v", err)
				os.Exit(1)
			}
		},
	}
	userModFlags := cmdUserMod.Cmd.Flags()
	userModFlags.StringP("user", "u", "", "The user id to be changed")
	userModFlags.StringP("groups", "g", "", "The groups to be set for the user")

	// the chmod command is used to change a group's permissions
	var cmdChMod x.SubCommand
	cmdChMod.Cmd = &cobra.Command{
		Use:   "chmod",
		Short: "Run Dgraph acl tool to change a group's permissions",
		Run: func(cmd *cobra.Command, args []string) {
			if err := chMod(cmdChMod.Conf); err != nil {
				glog.Errorf("Unable to change permisson for group:%v", err)
				os.Exit(1)
			}
		},
	}
	chModFlags := cmdChMod.Cmd.Flags()
	chModFlags.StringP("group", "g", "", "The group whose permission "+
		"is to be changed")
	chModFlags.StringP("pred", "p", "", "The predicates whose acls"+
		" are to be changed")
	chModFlags.IntP("perm", "P", 0, "The acl represented using "+
		"an integer, 4 for read-only, 2 for write-only, and 1 for modify-only")
	return []*x.SubCommand{
		&cmdUserAdd, &cmdUserDel, &cmdLogIn, &cmdGroupAdd, &cmdGroupDel, &cmdUserMod, &cmdChMod,
	}
}

func getDgraphClient(conf *viper.Viper) *dgo.Dgraph {
	opt = options{
		dgraph: conf.GetString("dgraph"),
	}
	glog.Infof("Running transaction with dgraph endpoint: %v", opt.dgraph)

	if len(opt.dgraph) == 0 {
		glog.Fatalf("The --dgraph option must be set in order to connect to dgraph")
	}

	x.LoadTLSConfig(&tlsConf, CmdAcl.Conf, x.TlsClientCert, x.TlsClientKey)
	tlsConf.ServerName = CmdAcl.Conf.GetString("tls_server_name")

	ds := strings.Split(opt.dgraph, ",")
	var clients []api.DgraphClient
	for _, d := range ds {
		conn, err := x.SetupConnection(d, &tlsConf)
		x.Checkf(err, "While trying to setup connection to Dgraph alpha.")

		dc := api.NewDgraphClient(conn)
		clients = append(clients, dc)
	}

	return dgo.NewDgraphClient(clients...)
}