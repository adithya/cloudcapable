package dockerutils

// CODE IS NOT ORGINALLY MINE, MODIFIED SOME OF IT
// SOURCE: https://github.com/docker/cli/blob/master/cli/command/container/cp.go
// SOURCE PROJECT: https://github.com/docker/cli

import (
	"archive/tar"
	"bytes"
	"context"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// ServerInfo stores details about the supported features and platform of the
// server
type ServerInfo struct {
	HasExperimental bool
	OSType          string
	BuildkitVersion types.BuilderVersion
}

// ClientInfo stores details about the supported features of the client
type ClientInfo struct {
	// Deprecated: experimental CLI features always enabled. This field is kept
	// for backward-compatibility, and is always "true".
	HasExperimental bool
	DefaultVersion  string
}

// CpConfig is a struct
type CpConfig struct {
	FollowLink bool
	CopyUIDGID bool
	Quiet      bool
	SourcePath string
	DestPath   string
	Container  string
}

// ConfigFile is a filename and the contents of the file as a Dict
type ConfigFile struct {
	Filename string
	Config   map[string]interface{}
}

func resolveLocalPath(localPath string) (absPath string, err error) {
	if absPath, err = filepath.Abs(localPath); err != nil {
		return
	}
	return archive.PreserveTrailingDotOrSeparator(absPath, localPath, filepath.Separator), nil
}

// CopyStringToContainer is a function
func CopyStringToContainer(ctx context.Context, cli *client.Client, copyConfig CpConfig, terraformInput string) (err error) {
	dstPath := copyConfig.DestPath

	client := cli
	// Prepare destination copy info by stat-ing the container path.
	dstInfo := archive.CopyInfo{Path: dstPath}
	dstStat, err := client.ContainerStatPath(ctx, copyConfig.Container, dstPath)

	// If the destination is a symbolic link, we should evaluate it.
	if err == nil && dstStat.Mode&os.ModeSymlink != 0 {
		linkTarget := dstStat.LinkTarget
		if !system.IsAbs(linkTarget) {
			// Join with the parent directory.
			dstParent, _ := archive.SplitPathDirEntry(dstPath)
			linkTarget = filepath.Join(dstParent, linkTarget)
		}

		dstInfo.Path = linkTarget
		dstStat, err = client.ContainerStatPath(ctx, copyConfig.Container, linkTarget)
	}

	// Validate the destination path
	if err := command.ValidateOutputPathFileMode(dstStat.Mode); err != nil {
		return errors.Wrapf(err, `destination "%s:%s" must be a directory or a regular file`, copyConfig.Container, dstPath)
	}

	// Ignore any error and assume that the parent directory of the destination
	// path exists, in which case the copy may still succeed. If there is any
	// type of conflict (e.g., non-directory overwriting an existing directory
	// or vice versa) the extraction will fail. If the destination simply did
	// not exist, but the parent directory does, the extraction will still
	// succeed.
	if err == nil {
		dstInfo.Exists, dstInfo.IsDir = true, dstStat.Mode.IsDir()
	}

	var (
		content         io.ReadCloser
		resolvedDstPath string
		// copiedSize      float64
	)

	// Make tar file
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var files = []struct {
		Name, Body string
	}{
		{"main.tf", terraformInput},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name: "main.tf",
			Mode: 0600,
			Size: int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			log.Fatal(err)
		}

		if _, err := tw.Write([]byte(file.Body)); err != nil {
			log.Fatal(err)
		}
	}

	if err == io.EOF {
		tw.Close()
		// w.Close()
	}

	if err != nil {
		// w.CloseWithError(err)
		log.Fatal(err)
	}

	// With the stat info about the local source as well as the
	// destination, we have enough information to know whether we need to
	// alter the archive that we upload so that when the server extracts
	// it to the specified directory in the container we get the desired
	// copy behavior.
	// See comments in the implementation of `archive.PrepareArchiveCopy`
	// for exactly what goes into deciding how and whether the source
	// archive needs to be altered for the correct copy behavior when it is
	// extracted. This function also infers from the source and destination
	// info which directory to extract to, which may be the parent of the
	// destination that the user specified.
	if err != nil {
		panic(err)
	}

	resolvedDstPath = "app"
	content = ioutil.NopCloser(bytes.NewReader(buf.Bytes()))

	options := types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
		CopyUIDGID:                copyConfig.CopyUIDGID,
	}

	if copyConfig.Quiet {
		return client.CopyToContainer(ctx, copyConfig.Container, resolvedDstPath, content, options)
	}

	res := client.CopyToContainer(ctx, copyConfig.Container, resolvedDstPath, content, options)

	return res
}

// Not used right now, but keeping in the event I might need for UTs
// Sample options
// options := cpConfig{
// 	followLink: false,
// 	copyUIDGID: false,
// 	quiet:      false,
// 	sourcePath: "terraformTestFiles/docker-nginx.tf",
// 	DestPath:   "app",
// 	container:  resp.ID,
// }
func copyToContainer(ctx context.Context, cli *client.Client, copyConfig CpConfig) (err error) {
	srcPath := copyConfig.SourcePath
	dstPath := copyConfig.DestPath

	if srcPath != "-" {
		// Get an absolute source path.
		srcPath, err = resolveLocalPath(srcPath)
		if err != nil {
			return err
		}
	}

	client := cli
	// Prepare destination copy info by stat-ing the container path.
	dstInfo := archive.CopyInfo{Path: dstPath}
	dstStat, err := client.ContainerStatPath(ctx, copyConfig.Container, dstPath)

	// If the destination is a symbolic link, we should evaluate it.
	if err == nil && dstStat.Mode&os.ModeSymlink != 0 {
		linkTarget := dstStat.LinkTarget
		if !system.IsAbs(linkTarget) {
			// Join with the parent directory.
			dstParent, _ := archive.SplitPathDirEntry(dstPath)
			linkTarget = filepath.Join(dstParent, linkTarget)
		}

		dstInfo.Path = linkTarget
		dstStat, err = client.ContainerStatPath(ctx, copyConfig.Container, linkTarget)
	}

	// Validate the destination path
	if err := command.ValidateOutputPathFileMode(dstStat.Mode); err != nil {
		return errors.Wrapf(err, `destination "%s:%s" must be a directory or a regular file`, copyConfig.Container, dstPath)
	}

	// Ignore any error and assume that the parent directory of the destination
	// path exists, in which case the copy may still succeed. If there is any
	// type of conflict (e.g., non-directory overwriting an existing directory
	// or vice versa) the extraction will fail. If the destination simply did
	// not exist, but the parent directory does, the extraction will still
	// succeed.
	if err == nil {
		dstInfo.Exists, dstInfo.IsDir = true, dstStat.Mode.IsDir()
	}

	var (
		content         io.ReadCloser
		resolvedDstPath string
		// copiedSize      float64
	)

	if srcPath == "-" {
		content = os.Stdin
		resolvedDstPath = dstInfo.Path
		if !dstInfo.IsDir {
			return errors.Errorf("destination \"%s:%s\" must be a directory", copyConfig.Container, dstPath)
		}
	} else {
		// Prepare source copy info.
		srcInfo, err := archive.CopyInfoSourcePath(srcPath, copyConfig.FollowLink)
		if err != nil {
			return err
		}

		srcArchive, err := archive.TarResource(srcInfo)
		if err != nil {
			return err
		}
		defer srcArchive.Close()

		// With the stat info about the local source as well as the
		// destination, we have enough information to know whether we need to
		// alter the archive that we upload so that when the server extracts
		// it to the specified directory in the container we get the desired
		// copy behavior.

		// See comments in the implementation of `archive.PrepareArchiveCopy`
		// for exactly what goes into deciding how and whether the source
		// archive needs to be altered for the correct copy behavior when it is
		// extracted. This function also infers from the source and destination
		// info which directory to extract to, which may be the parent of the
		// destination that the user specified.
		dstDir, preparedArchive, err := archive.PrepareArchiveCopy(srcArchive, srcInfo, dstInfo)
		if err != nil {
			panic(err)
		}
		defer preparedArchive.Close()

		resolvedDstPath = dstDir
		content = preparedArchive
	}

	options := types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
		CopyUIDGID:                copyConfig.CopyUIDGID,
	}

	if copyConfig.Quiet {
		return client.CopyToContainer(ctx, copyConfig.Container, resolvedDstPath, content, options)
	}

	res := client.CopyToContainer(ctx, copyConfig.Container, resolvedDstPath, content, options)

	return res
}
