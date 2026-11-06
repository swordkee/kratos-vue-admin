package media

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"github.com/swordkee/kratos-vue-admin/pkg/log"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/media"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
)

// aliyunStorage 阿里云 OSS 存储（复用骨架已引入的 aliyun-oss-go-sdk，不新增依赖）。
type aliyunStorage struct {
	bucket *oss.Bucket
	domain string
	l      *log.Helper
}

// newAliyunStorage 构造 OSS 存储；endpoint/ak/sk/bucket 缺失时直接报错（装配期 fail-closed）。
func newAliyunStorage(c *conf.OssConfig, logger log.Logger) (media.StorageRepo, error) {
	if c.Endpoint == "" || c.AccessKey == "" || c.AccessSecret == "" || c.BucketName == "" {
		return nil, errors.New("oss.aliyun 配置不完整（endpoint/accessKey/accessSecret/bucketName）")
	}
	client, err := oss.New(c.Endpoint, c.AccessKey, c.AccessSecret)
	if err != nil {
		return nil, err
	}
	bucket, err := client.Bucket(c.BucketName)
	if err != nil {
		return nil, err
	}
	return &aliyunStorage{
		bucket: bucket,
		domain: strings.TrimSuffix(c.ImgDomain, "/"),
		l:      log.NewHelper(log.With(logger, "module", "app/admin/internal/data/media/aliyun")),
	}, nil
}

func (s *aliyunStorage) Mode() string { return "aliyun" }

// URL 访问地址 = imgDomain + "/" + key（imgDomain 为空时退回 bucketURL，再退回相对 key）。
func (s *aliyunStorage) URL(key string) string {
	if s.domain == "" {
		return key
	}
	return s.domain + "/" + key
}

func (s *aliyunStorage) Save(_ context.Context, key string, r io.Reader, _ int64) error {
	if err := media.ValidateKey(key); err != nil {
		return err
	}
	return s.bucket.PutObject(key, r)
}

func (s *aliyunStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if err := media.ValidateKey(key); err != nil {
		return nil, err
	}
	body, err := s.bucket.GetObject(key)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (s *aliyunStorage) Delete(_ context.Context, key string) error {
	if err := media.ValidateKey(key); err != nil {
		return err
	}
	exists, err := s.bucket.IsObjectExist(key)
	if err != nil {
		return err
	}
	if !exists {
		return media.ErrObjectNotFound
	}
	return s.bucket.DeleteObject(key)
}
