package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

const (
	// exporterPath is the path to the script under test.  You must mount this
	// path into the test container yourself.
	exporterPath = "/inotify-instances"

	fswatchPath = "/usr/local/bin/fswatch"
)

type stopper interface {
	Stop() error
}

type multiStopper struct {
	s []stopper
}

func (s *multiStopper) Stop() error {
	var firstError error
	for i := range s.s {
		if err := s.s[i].Stop(); err != nil && firstError == nil {
			firstError = err
		}
	}
	return firstError
}

type watcher struct {
	cmd     *exec.Cmd
	control io.WriteCloser
}

func (w *watcher) Stop() error {
	if _, err := fmt.Fprintln(w.control, "die"); err != nil {
		return err
	}
	w.control.Close()
	w.cmd.Wait()
	return nil
}

func runWatcherUnprivileged() (stopper, error) {
	return runWatcher(fswatchPath, "/")
}

func runWatcherPrivileged() (stopper, error) {
	return runWatcher(sudoArgv(fswatchPath, "/")...)
}

func runWatcher(argv ...string) (stopper, error) {
	// e2e -> fswatch
	outboundReader, outboundWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer outboundReader.Close()

	// fswatch -> e2e
	inboundReader, inboundWriter, err := os.Pipe()
	if err != nil {
		outboundWriter.Close()
		return nil, err
	}
	defer inboundReader.Close()
	defer inboundWriter.Close()

	c := argvCommand(argv...)
	c.Stderr = os.Stderr
	c.ExtraFiles = []*os.File{outboundReader, inboundWriter}
	if err := c.Start(); err != nil {
		outboundWriter.Close()
		return nil, err
	}

	// Wait for fswatch to open its inotify instance.
	inboundWriter.Close()
	b := make([]byte, 6)
	inboundReader.Read(b)

	return &watcher{
		cmd:     c,
		control: outboundWriter,
	}, nil
}

func runExporterUnprivileged() ([]byte, error) {
	return runExporter(exporterPath)
}

func runExporterPrivileged() ([]byte, error) {
	return runExporter(sudoArgv(exporterPath)...)
}

func runExporter(argv ...string) ([]byte, error) {
	c := argvCommand(argv...)
	c.Stderr = os.Stderr
	return c.Output()
}

func sudoArgv(argv ...string) []string {
	if len(argv) < 1 {
		panic("argv is empty")
	}
	sudo := []string{"sudo", "-n", "-C", "5"}
	sudo = append(sudo, argv...)
	return sudo
}

func argvCommand(argv ...string) *exec.Cmd {
	if len(argv) < 1 {
		panic("argv is empty")
	}
	name := argv[0]
	args := argv[1:]
	return exec.Command(name, args...)
}

func decodeExporterOutput(output []byte) ([]*dto.MetricFamily, error) {
	d := expfmt.NewDecoder(bytes.NewReader(output), expfmt.FmtText)
	mfs := make([]*dto.MetricFamily, 0)
decode:
	for {
		mf := dto.MetricFamily{}
		if err := d.Decode(&mf); err != nil {
			if err == io.EOF {
				break decode
			}
			return mfs, err
		}
		mfs = append(mfs, &mf)
	}
	return mfs, nil
}
