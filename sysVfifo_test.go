package hhfifo

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func TestQueueLifecycle(t *testing.T) {
	cfg := Config{
		Key:      0x1234,
		MaxSize:  10240,
		MaxSizeQ: 4196000,
		MsgType:  100,
		Block:    false,
		MaxConns: 10000,
		Perm:     0666,
	}

	// 测试队列创建
	q, err := NewQueue(cfg)
	assert.NoError(t, err)
	defer q.Close()

	// 测试基本读写
	msg := []byte("test message")
	assert.NoError(t, q.Enqueue(msg))

	result, err := q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, msg, result)

	// 测试并发读写
	var wg sync.WaitGroup
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			assert.NoError(t, q.Enqueue([]byte(fmt.Sprintf("msg%d", n))))
		}(i)
	}
	wg.Wait()

	count := 0
	for {
		_, err := q.Dequeue()
		if err != nil {
			assert.Contains(t, err.Error(), "no message available")
			break
		}
		count++
	}
	assert.Equal(t, 10000, count)
}

func BenchmarkQueue(b *testing.B) {
	cfg := Config{
		Key:      0x1234,
		MaxSize:  10240,
		MaxSizeQ: 4196000,
		MsgType:  100,
		Block:    false,
		MaxConns: 1000,
		Perm:     0666,
	}
	q, _ := NewQueue(cfg)
	defer q.Close()

	b.Run("Single", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = q.Enqueue([]byte("test"))
			_, _ = q.Dequeue()
		}
	})

}
