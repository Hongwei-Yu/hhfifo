package hhfifo

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// 系统常量定义（Linux）
const (
	IPC_CREAT  = 01000
	IPC_EXCL   = 02000
	IPC_RMID   = 0
	IPC_SET    = 1
	IPC_STAT   = 2
	IPC_INFO   = 3
	IPC_NOWAIT = 04000
)
const (
	MSG_NOERROR = 010000
	MSG_EXCEPT  = 020000
	MSG_COPY    = 040000
)

const (
	URW  = 0600
	ARW  = 0666
	UWOR = 0644
)

var (
	ErrNoMessage      = errors.New("no message available")
	ErrQueueDestroyed = errors.New("message queue destroyed")
)

// MessageQueue 消息队列结构体
type MessageQueue struct {
	id     int
	mu     sync.Mutex
	sem    *highPerfSem
	pool   *sync.Pool
	config Config
}

type Config struct {
	Key      int    // IPC key
	MaxSize  int    // 最大消息长度（字节）
	MaxSizeQ int    // 消息队列最大长度（字节）
	MsgType  int64  // 默认消息类型
	Block    bool   // 是否阻塞模式
	MaxConns int32  // 最大并发连接数
	Flags    int    // 创建标志
	Perm     uint32 // 权限模式
}

type sysVsdMsg struct {
	mtype int64
	mtext [1024]byte
}

func NewQueue(cfg Config) (*MessageQueue, error) {
	if cfg.MaxSize < 1 || cfg.MaxSize > 64*1024 {
		return nil, fmt.Errorf("MaxSize must be 1-65536")
	}

	qid, _, errno := syscall.Syscall(
		syscall.SYS_MSGGET,
		uintptr(cfg.Key),
		uintptr(cfg.Flags|IPC_CREAT|int(cfg.Perm)),
		0,
	)
	if errno != 0 {
		return nil, handleErrno(errno, "msgget", cfg.Key)
	}

	mq := &MessageQueue{
		id:     int(qid),
		sem:    newHighPerfSem(cfg.MaxConns),
		config: cfg,
		pool: &sync.Pool{
			New: func() interface{} {
				return sysVsdMsg{}
			},
		},
	}

	//初始化队列状态
	//msgctl := NewMsgCtl(int(qid))
	//ds, err := msgctl.Stat()
	//if err != nil {
	//	return nil, err
	//}
	//ds.MsgQbytes = uint64(cfg.MaxSizeQ)
	//
	//err = msgctl.Set(ds)
	//if err != nil {
	//	return nil, err
	//}
	//ds, _ = msgctl.Stat()
	//
	//log.Println()

	runtime.SetFinalizer(mq, func(q *MessageQueue) {
		err := q.Close()
		if err != nil {
			return
		}
	})
	return mq, nil
}

func (q *MessageQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, _, errno := syscall.Syscall(
		syscall.SYS_MSGCTL,
		uintptr(q.id),
		uintptr(IPC_RMID),
		0,
	); errno != 0 {
		return handleErrno(errno, "msgctl(RMID)")
	}
	return nil
}

func (q *MessageQueue) Enqueue(msg []byte) error {
	if len(msg) > q.config.MaxSize {
		return fmt.Errorf("message size %d exceeds limit %d", len(msg), q.config.MaxSize)
	}

	msgStruct := q.pool.Get().(sysVsdMsg)
	msgStruct.mtype = q.config.MsgType
	copy(msgStruct.mtext[:], msg)
	defer q.pool.Put(msgStruct)

	q.mu.Lock()
	defer q.mu.Unlock()

	_, _, errno := syscall.Syscall6(
		syscall.SYS_MSGSND,
		uintptr(q.id),
		uintptr(unsafe.Pointer(&msgStruct)),
		uintptr(len(msg)),
		uintptr(IPC_NOWAIT),
		0, 0,
	)
	//fmt.Println("send:", msgStruct)
	return handleErrno(errno, "msgsnd")
}

func (q *MessageQueue) Dequeue() ([]byte, error) {
	q.sem.Acquire()
	defer q.sem.Release()

	// 获取消息缓冲区
	msg := q.pool.Get().(sysVsdMsg)
	defer q.pool.Put(msg)

	// 配置接收参数
	flags := MSG_NOERROR
	if !q.config.Block {
		flags |= IPC_NOWAIT
	}

	// 系统调用
	var n uintptr
	var errno syscall.Errno
	for { // 处理EINTR
		n, _, errno = syscall.Syscall6(
			syscall.SYS_MSGRCV,
			uintptr(q.id),
			uintptr(unsafe.Pointer(&msg)),
			unsafe.Sizeof(msg.mtext),
			uintptr(q.config.MsgType),
			uintptr(flags),
			0,
		)

		if errno != syscall.EINTR {
			break
		}
	}

	// 错误处理
	if errno != 0 {
		if errno == syscall.ENOMSG {
			return nil, ErrNoMessage
		}
		return nil, handleErrno(errno, "msgrcv")
	}

	// 解析数据
	dataSize := n

	if dataSize > uintptr(q.config.MaxSize) {
		return nil, fmt.Errorf("message size %d exceeds config", dataSize)
	}
	//fmt.Println("type", msg.mtype)
	//fmt.Println("text", msg.mtext)

	result := make([]byte, dataSize)
	copy(result, msg.mtext[:dataSize])
	//fmt.Println("recv:", result)
	return result, nil
}

func handleErrno(errno syscall.Errno, operation string, args ...interface{}) error {
	if errno == 0 {
		return nil
	}

	// 错误码映射
	errMap := map[syscall.Errno]string{
		syscall.ENOMSG: "no message of desired type",
		syscall.E2BIG:  "message too large",
		syscall.EACCES: "permission denied",
		syscall.EIDRM:  "queue was removed",
		syscall.ENOSPC: "queue limit reached",
		syscall.ENOENT: "queue does not exist",
		syscall.EINVAL: "invalid parameters",
		syscall.EFAULT: "invalid ds buffer segment address",
		syscall.EPERM:  "permission denied",
	}

	desc, ok := errMap[errno]
	if !ok {
		desc = errno.Error()
	}

	ctx := ""
	if len(args) > 0 {
		ctx = fmt.Sprintf(" [ctx: %v]", args)
	}

	return fmt.Errorf("%s failed: %s (code=%d)%s",
		operation, desc, int(errno), ctx)
}

// 辅助方法
//func (q *MessageQueue) Stats() syscall.Msginfo {
//	return q.qstat
//}

//func (q *MessageQueue) IsAlive() bool {
//	_, _, errno := syscall.Syscall(
//		syscall.SYS_MSGCTL,
//		uintptr(q.id),
//		uintptr(IPC_STAT),
//		uintptr(unsafe.Pointer(&q.qstat)),
//	)
//	return errno == 0
//}
