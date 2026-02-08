package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

var podBucket = []byte("pods")

// Store 存储接口
type Store struct {
	db *bbolt.DB
}

// NewStore 创建新的存储实例
func NewStore(path string) (*Store, error) {
	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(podBucket)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	return &Store{db: db}, nil
}

// Close 关闭存储
func (s *Store) Close() error {
	return s.db.Close()
}

// Put 存储对象
func (s *Store) Put(key string, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(podBucket)
		return b.Put([]byte(key), data)
	})
}

// Get 获取对象
func (s *Store) Get(key string, obj interface{}) error {
	return s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(podBucket)
		data := b.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("not found: %s", key)
		}
		return json.Unmarshal(data, obj)
	})
}

// Delete 删除对象
func (s *Store) Delete(key string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(podBucket)
		return b.Delete([]byte(key))
	})
}

// List 列出所有对象
func (s *Store) List(factory func() interface{}, callback func(interface{})) error {
	return s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(podBucket)
		return b.ForEach(func(k, v []byte) error {
			obj := factory()
			if err := json.Unmarshal(v, obj); err != nil {
				return err
			}
			callback(obj)
			return nil
		})
	})
}
