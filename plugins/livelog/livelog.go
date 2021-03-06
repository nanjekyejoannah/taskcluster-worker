// Package livelog implements a webhook handler for serving up livelogs of a task
// sandbox.
package livelog

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

type pluginProvider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
	log         *logrus.Entry
	environment *runtime.Environment
}

type taskPlugin struct {
	plugins.TaskPluginBase
	context     *runtime.TaskContext
	url         string
	expiration  tcclient.Time
	log         *logrus.Entry
	environment *runtime.Environment
}

func (pluginProvider) NewPlugin(opts plugins.PluginOptions) (plugins.Plugin, error) {
	return plugin{
		log:         opts.Log,
		environment: opts.Environment,
	}, nil
}

func (p plugin) NewTaskPlugin(opts plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	return &taskPlugin{
		TaskPluginBase: plugins.TaskPluginBase{},
		log: p.log.WithFields(logrus.Fields{
			"taskID": opts.TaskInfo.TaskID,
			"runID":  opts.TaskInfo.RunID,
		}),
		environment: p.environment,
	}, nil
}

func (tp *taskPlugin) Prepare(context *runtime.TaskContext) error {
	tp.context = context

	tp.url = tp.context.AttachWebHook(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO (garndt): add support for range headers.  Might not be used at all currently
		logReader, err := tp.context.NewLogReader()
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Error opening up live log"))
			return
		}
		defer logReader.Close()

		// Get an HTTP flusher if supported in the current context, or wrap in
		// a NopFlusher, if flushing isn't available.
		wf, ok := w.(ioext.WriteFlusher)
		if !ok {
			wf = ioext.NopFlusher(w)
		}

		ioext.CopyAndFlush(wf, logReader, 100*time.Millisecond)
	}))

	err := runtime.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     "public/logs/live.log",
		Mimetype: "text/plain",
		URL:      tp.url,
		Expires:  tp.context.TaskInfo.Expires,
	}, tp.context)
	if err != nil {
		tp.context.LogError(fmt.Sprintf("Could not initialize live log plugin. Error: %s", err))
	}

	return err
}

func (tp *taskPlugin) Finished(success bool) error {
	file, err := tp.context.ExtractLog()
	if err != nil {
		return err
	}
	defer file.Close()

	tempFile, err := tp.environment.TemporaryStorage.NewFile()
	if err != nil {
		return err
	}

	defer tempFile.Close()

	zip := gzip.NewWriter(tempFile)
	if _, err = io.Copy(zip, file); err != nil {
		return err
	}

	if err = zip.Close(); err != nil {
		return err
	}

	_, err = tempFile.Seek(0, 0)
	if err != nil {
		return err
	}

	err = runtime.UploadS3Artifact(runtime.S3Artifact{
		Name:     "public/logs/live_backing.log",
		Mimetype: "text/plain",
		Expires:  tp.context.TaskInfo.Expires,
		Stream:   tempFile,
		AdditionalHeaders: map[string]string{
			"Content-Encoding": "gzip",
		},
	}, tp.context)

	if err != nil {
		return err
	}

	backingURL := fmt.Sprintf("https://queue.taskcluster.net/v1/task/%s/runs/%d/artifacts/public/logs/live_backing.log", tp.context.TaskInfo.TaskID, tp.context.TaskInfo.RunID)
	err = runtime.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     "public/logs/live.log",
		Mimetype: "text/plain",
		URL:      backingURL,
		Expires:  tp.context.TaskInfo.Expires,
	}, tp.context)
	if err != nil {
		tp.log.Error(err)
		return err
	}

	return nil
}

func init() {
	plugins.Register("livelog", &pluginProvider{})
}
