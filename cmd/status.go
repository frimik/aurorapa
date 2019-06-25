/*Package cmd contains all commands.
Copyright © 2019 Mikael Fridh <frimik@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/google/logger"
	"github.com/gookit/color"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:     "status [/path/to/file.aurora]",
	Short:   "Summarize status for all jobs in aurora file.",
	Example: " status /jobs/app.aurora",
	Args:    cobra.ExactArgs(1),
	RunE:    statusCmdF,
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// statusCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// statusCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// JobUpdate contains info on a potential job update
type JobUpdate struct {
	JobIndex    int
	Job         string
	Dirty       bool
	Diff        []string
	FoundHeader bool
	Update      string
	Add         string
	Remove      string
}

// NewJobUpdate initializes a new JobUpdate with a default Dirty value set to false
func NewJobUpdate(job string, jobindex int) JobUpdate {
	jobupdate := JobUpdate{}
	jobupdate.Job = job
	jobupdate.JobIndex = jobindex
	// set defaults
	jobupdate.Dirty = false
	jobupdate.FoundHeader = false

	return jobupdate
}

func statusCmdF(command *cobra.Command, args []string) error {

	auroraFile := args[0]

	// unified is a nicer diff view
	// ignore `owner` differences
	os.Setenv("DIFF_VIEWER", "diff -u -I \"'owner': Identity\"")

	logger := logger.Init("aurorapa", true, false, ioutil.Discard)
	defer logger.Close()

	auroraExe := "aurora"

	auroraExePath, err := exec.LookPath(auroraExe)
	if err != nil {
		logger.Fatalf("%s not found", auroraExe)
	}
	//logger.Infof("%s is available at %s\n", command, path)

	cmd := exec.Command(auroraExePath, "config", "list", auroraFile)
	//cmd.Stdin = strings.NewReader("")
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()

	if err != nil {
		log.Fatal(err)
	}

	str1 := out.String()
	re := regexp.MustCompile(`\[([^\[\]]*)\]`)
	submatch := re.FindString(str1)

	jobseparators := "[], "
	f := func(r rune) bool {
		return strings.ContainsRune(jobseparators, r)
	}

	jobs := strings.FieldsFunc(submatch, f)
	//logger.Infof("Jobs: %q\n", jobs)

	fmt.Printf("Aurora file: %s contains %d jobs.\n", auroraFile, len(jobs))

	for i, job := range jobs {
		j := NewJobUpdate(job, i)

		diffCmd := exec.Command(auroraExePath, "job", "diff", j.Job, auroraFile)
		var out bytes.Buffer
		diffCmd.Stdout = &out
		err = diffCmd.Run()
		if err != nil {
			log.Fatal(err)
		}

		//fmt.Printf("%q\n", out.String())

		scanner := bufio.NewScanner(strings.NewReader(out.String()))

		for l := 0; scanner.Scan(); l++ {
			//fmt.Println(l, scanner.Text())
			if strings.HasPrefix(scanner.Text(), "This job update will:") {
				j.FoundHeader = true
			}

			if strings.HasPrefix(scanner.Text(), "remove instances:") {
				j.Dirty = true
				instances := re.FindString(scanner.Text())
				j.Remove = instances
			} else if strings.HasPrefix(scanner.Text(), "add instances:") {
				j.Dirty = true
				instances := re.FindString(scanner.Text())
				j.Add = instances
			} else if strings.HasPrefix(scanner.Text(), "update instances:") {
				instances := re.FindString(scanner.Text())
				j.Update = instances
			} else if !j.FoundHeader {
				j.Diff = append(j.Diff, scanner.Text())
				j.Dirty = true
			}

		}

		if j.Dirty {
			color.Notice.Printf(
				"# Job %d: %s, Dirty: %t. Remove: %s, Add: %s, Update: %s (Diff: %d lines)\n",
				j.JobIndex, j.Job, j.Dirty, j.Remove, j.Add, j.Update, len(j.Diff))

		} else {
			color.Success.Printf(
				"# Job %d: %s, Dirty: %t. Remove: %s, Add: %s, Update: %s (Diff: %d lines)\n",
				j.JobIndex, j.Job, j.Dirty, j.Remove, j.Add, j.Update, len(j.Diff))
		}

		for _, l := range j.Diff {
			fmt.Println(l)
		}

	}

	/* `aurora job diff` output sample:
		This job update will:
	update instances: [0-2]
	with diff:

	59c59,60
	<   -com.twitter.finagle.netty3.numWorkers=3 \\\\\
	---
	>   -com.twitter.finagle.netty3.numWorkers=6 \\\\\
	*/

	return nil
}