// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package miniorunner_test

import (
	"bytes"
	"context"
	"io"
	"slices"
	"testing"

	minioenv "github.com/amidgo/testenv/minio"

	miniorunner "github.com/amidgo/testenv/minio/runner"

	"github.com/minio/minio-go/v7"
)

type RunForTestingTest struct {
	CaseName string
	Content  []minioenv.Bucket
}

func (r *RunForTestingTest) Name() string {
	return r.CaseName
}

func (r *RunForTestingTest) Test(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	minioClient := miniorunner.RunForTesting(t, r.Content...)

	_, err := minioClient.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("get list of buckets, unexpected error: %+v", err)
	}

	for _, bucket := range r.Content {
		exists, err := minioClient.BucketExists(ctx, bucket.Name)
		if err != nil {
			t.Fatalf("check bucket %s exists, unexpected error: %+v", bucket, err)
		}

		if !exists {
			t.Fatalf("check bucket %s exists, bucket not exists", bucket)
		}

		for _, initialFile := range bucket.Files {
			requireInitialFileExists(t, ctx, minioClient, bucket.Name, initialFile)
		}
	}
}

func requireInitialFileExists(
	t *testing.T,
	ctx context.Context,
	minioClient *minio.Client,
	bucketName string,
	initialFile minioenv.File,
) {
	object, err := minioClient.GetObject(
		ctx,
		bucketName,
		initialFile.Name,
		minio.GetObjectOptions{},
	)
	if err != nil {
		t.Fatalf(
			"get object %s from bucket %s, unexpected error: %+v",
			initialFile.Name,
			bucketName,
			err,
		)
	}

	objectData := &bytes.Buffer{}

	_, err = io.Copy(objectData, object)
	if err != nil {
		t.Fatalf(
			"read data from object %s from bucket %s, unexpected error: %+v",
			initialFile.Name,
			bucketName,
			err,
		)
	}

	if !slices.Equal(objectData.Bytes(), initialFile.Content) {
		t.Fatalf(
			"objectData from %s from bucket %s not equal,\nexpected:\n\t%s\nactual:\n\t%s",
			initialFile.Name,
			bucketName,
			initialFile.Content,
			objectData.String(),
		)
	}
}

func runForTestingTests(
	t *testing.T,
	tests ...*RunForTestingTest,
) {
	for _, tst := range tests {
		t.Run(tst.Name(), tst.Test)
	}
}

func Test_Minio(t *testing.T) {
	t.Parallel()

	runForTestingTests(t,
		&RunForTestingTest{
			CaseName: "empty buckets and files",
		},
		&RunForTestingTest{
			CaseName: "only buckets filled",
			Content: []minioenv.Bucket{
				{
					Name: "first",
				},
				{
					Name: "first",
				},
				{
					Name: "second",
				},
				{
					Name: "third",
				},
			},
		},
		&RunForTestingTest{
			CaseName: "all filled, no conflicts",
			Content: []minioenv.Bucket{
				{
					Name: "first",
					Files: []minioenv.File{
						{
							Name:    "Gagarin.txt",
							Content: []byte("Поехали!"),
						},
					},
				},
				{
					Name: "second",
					Files: []minioenv.File{
						{
							Name:    "Titov.txt",
							Content: []byte("Второй..."),
						},
					},
				},
			},
		},
	)
}
