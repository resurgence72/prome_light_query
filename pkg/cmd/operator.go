package cmd

import (
	"fmt"
	"os/exec"
)

func (c *cmd) Push(
	localPath string,
	remotePath string,
) error {
	var es []error
	fmt.Println("aa ",c.ps)
	for _, p := range c.ps {
		err := exec.CommandContext(
			c.timeout, "/bin/bash", "-c",
			fmt.Sprintf(
				"sshpass -p %s scp -p -o StrictHostKeyChecking=no %s %s@%s:%s",
				c.password,
				localPath,
				c.username,
				p,
				remotePath,
			),
		).Run()
		if err != nil {
			es = append(es, err)
		}
	}

	if len(es) == 0 {
		return nil
	}
	return es[0]
}

func (c *cmd) Pull(
	remotePath string,
	localPath string,
) error {
	var es []error
	fmt.Println("bb ",c.ps)

	for _, p := range c.ps {
		err := exec.CommandContext(
			c.timeout, "/bin/bash", "-c",
			fmt.Sprintf(
				c.command,
				c.password,
				c.username,
				p,
				remotePath,
				fmt.Sprintf("%s_%s", localPath, p),
			),
		).Run()

		if err != nil {
			es = append(es, err)
		}
	}

	if len(es) == 0 {
		return nil
	}
	return es[0]
}
