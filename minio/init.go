// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package minioenv

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"path"

	"github.com/minio/minio-go/v7"
	minioclient "github.com/minio/minio-go/v7"
)

type Bucket struct {
	Name  string
	Files []File
}

type File struct {
	Name    string
	Content []byte
}

func MustFiles(fsys fs.FS) []File {
	files, err := Files(fsys)
	if err != nil {
		panic(err)
	}

	return files
}

func Files(fsys fs.FS) ([]File, error) {
	filePaths, err := fs.Glob(fsys, "*")
	if err != nil {
		return nil, fmt.Errorf("glob files by pattern, %w", err)
	}

	files := make([]File, 0, len(filePaths))

	err = fs.WalkDir(fsys, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		content, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return fmt.Errorf("read file, %W", err)
		}

		_, name := path.Split(filePath)

		files = append(files, File{Name: name, Content: content})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk dir, %w", err)
	}

	return files, nil
}

func Init(
	ctx context.Context,
	env Environment,
	buckets ...Bucket,
) (minioClient *minio.Client, term func(), err error) {
	term = func() {
		terminateErr := env.Terminate(ctx)
		if terminateErr != nil {
			log.Printf("failed to terminate minio environment: %s", terminateErr)
		}
	}

	minioClient, err = env.Connect(ctx)
	if err != nil {
		return nil, term, fmt.Errorf("connect to minio environment, %w", err)
	}

	err = insertBuckets(ctx, minioClient, buckets...)
	if err != nil {
		return nil, term, err
	}

	return minioClient, term, nil
}

func insertBuckets(ctx context.Context, minioClient *minio.Client, buckets ...Bucket) error {
	for _, bucket := range buckets {
		err := insertSingleBucket(ctx, minioClient, bucket)
		if err != nil {
			return err
		}
	}

	return nil
}

func insertSingleBucket(ctx context.Context, minioClient *minio.Client, bucket Bucket) error {
	makeBucketOpts := minioclient.MakeBucketOptions{}

	err := minioClient.MakeBucket(ctx, bucket.Name, makeBucketOpts)

	switch {
	case isBucketExistsError(err):
	case err != nil:
		return fmt.Errorf("create bucket %s, %w", bucket.Name, err)
	}

	putObjectOpts := minioclient.PutObjectOptions{}

	for _, file := range bucket.Files {
		objectSize := int64(len(file.Content))

		_, err = minioClient.PutObject(ctx,
			bucket.Name,
			file.Name,
			bytes.NewBuffer(file.Content),
			objectSize,
			putObjectOpts,
		)
		if err != nil {
			return fmt.Errorf("put file %s into bucket %s, %w", file.Name, bucket.Name, err)
		}
	}

	return nil
}

func isBucketExistsError(err error) bool {
	resp := minio.ToErrorResponse(err)

	return resp.Code == "BucketAlreadyOwnedByYou"
}
