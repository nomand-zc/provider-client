package queue

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestChainQueue(t *testing.T) {
	// 测试创建队列
	q := NewChainQueue[int](10)

	// 测试Closed方法
	if q.Closed() {
		t.Error("New queue should not be closed")
	}

	// 测试Len方法 - 空队列
	if q.Len() != 0 {
		t.Errorf("Empty queue should have length 0, got %d", q.Len())
	}

	// 测试Write方法
	err := q.Write(42)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	// 测试Len方法 - 写入后
	if q.Len() != 1 {
		t.Errorf("Queue should have length 1 after write, got %d", q.Len())
	}

	// 测试Read方法
	item, err := q.Read()
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}
	if item != 42 {
		t.Errorf("Expected 42, got %d", item)
	}

	// 测试Len方法 - 读取后
	if q.Len() != 0 {
		t.Errorf("Queue should have length 0 after read, got %d", q.Len())
	}

	// 测试批量写入
	for i := range 5 {
		err = q.Write(i)
		if err != nil {
			t.Errorf("Write %d failed: %v", i, err)
		}
	}

	// 测试Len方法 - 批量写入后
	if q.Len() != 5 {
		t.Errorf("Queue should have length 5 after batch write, got %d", q.Len())
	}

	// 测试Close方法
	err = q.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// 测试关闭后状态
	if !q.Closed() {
		t.Error("Queue should be closed after Close()")
	}

	// 测试关闭后写入
	err = q.Write(100)
	if err == nil {
		t.Error("Write should fail on closed queue")
	}

	// 测试关闭后读取 - 队列中还有数据，应该能成功读取
	for i := range 5 {
		item, err = q.Read()
		if err != nil {
			t.Errorf("Read should succeed on closed queue with data, got error: %v", err)
		}
		if item != i {
			t.Errorf("Expected %d, got %d", i, item)
		}
	}

	// 测试Len方法 - 读取所有数据后
	if q.Len() != 0 {
		t.Errorf("Queue should have length 0 after reading all data, got %d", q.Len())
	}

	// 测试关闭后读取空队列
	_, err = q.Read()
	if err == nil {
		t.Error("Read should fail on closed empty queue")
	}

	fmt.Println("All queue tests passed!")
}

// TestConcurrentAccess 测试并发读写场景
func TestConcurrentAccess(t *testing.T) {
	q := NewChainQueue[int](100)
	var wg sync.WaitGroup
	
	// 启动10个写入goroutine
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				err := q.Write(id*100 + j)
				if err != nil && !IsClosedError(err) && !IsFullError(err) {
					t.Errorf("Unexpected error from goroutine %d: %v", id, err)
				}
				// 如果是队列满错误，稍等再试
				if IsFullError(err) {
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}
	
	// 启动10个读取goroutine
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, err := q.Read()
				if err != nil && !IsClosedError(err) {
					t.Errorf("Read error from goroutine %d: %v", id, err)
				}
			}
		}(i)
	}
	
	// 等待所有goroutine完成
	wg.Wait()
	
	// 验证队列状态
	if q.Len() != 0 {
		t.Errorf("Queue should be empty after concurrent access, got length %d", q.Len())
	}
	
	fmt.Println("Concurrent access test passed!")
}

// TestConcurrentClose 测试并发关闭场景
func TestConcurrentClose(t *testing.T) {
	q := NewChainQueue[int](10)
	var wg sync.WaitGroup
	
	// 启动多个goroutine同时尝试关闭
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := q.Close()
			if err != nil && !IsClosedError(err) {
				t.Errorf("Close error from goroutine %d: %v", id, err)
			}
		}(i)
	}
	
	// 同时启动读写goroutine
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				err := q.Write(id*10 + j)
				if err != nil && !IsClosedError(err) && !IsFullError(err) {
					t.Errorf("Write error: %v", err)
				}
				
				_, err = q.Read()
				if err != nil && !IsClosedError(err) {
					t.Errorf("Read error: %v", err)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// 验证队列已关闭
	if !q.Closed() {
		t.Error("Queue should be closed after concurrent close attempts")
	}
	
	fmt.Println("Concurrent close test passed!")
}

// TestQueueFull 测试队列满的情况
func TestQueueFull(t *testing.T) {
	q := NewChainQueue[int](5) // 小缓冲区
	
	// 填满队列
	for i := 0; i < 5; i++ {
		err := q.Write(i)
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
	}
	
	// 尝试写入第6个元素，应该返回队列满错误
	err := q.Write(6)
	if !IsFullError(err) {
		t.Errorf("Expected queue full error, got: %v", err)
	}
	
	fmt.Println("Queue full test passed!")
}
