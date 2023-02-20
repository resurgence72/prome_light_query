package cmd

import (
	"context"
	"time"
)

type cmd struct {
	username string
	password string
	ps       []string
	timeout  context.Context
	command  string
}

var Cmd *cmd

func NewCmdRunner(
	username string,
	password string,
	ps []string,
	timeout time.Duration,
) *cmd {
	ctx, _ := context.WithTimeout(context.TODO(), 5*time.Second)

	/*
		apt-get -y install sshpass > /dev/null 2>&1 ; yum -y install sshpass > /dev/null 2>&1 ;
		sshpass -p 1 scp -p -o StrictHostKeyChecking=no root@10.0.0.105:/root/prometheus-2.41.0.linux-amd64/log/prometheus.log ./p8s.log
	*/
	before := "apt-get -y install sshpass > /dev/null 2>&1 ; yum -y install sshpass > /dev/null 2>&1 ;"
	//exec.CommandContext(
	//	ctx,
	//	"/bin/bash",
	//	"-c",
	//	fmt.Sprintf("sshpass -p %s scp -p -o StrictHostKeyChecking=no %s@%s:%s %s"),
	//)

	Cmd = &cmd{
		username: username,
		password: password,
		ps:       ps,
		timeout:  ctx,
		command:  before + "sshpass -p %s scp -p -o StrictHostKeyChecking=no %s@%s:%s %s",
	}
	return Cmd
}
