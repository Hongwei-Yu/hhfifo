package hhfifo

import (
	"syscall"
	"unsafe"
)

// MsqidDs 对应内核的 msqid_ds 结构体（64位系统）
type MsqidDs struct {
	MsgPerm   MsqPerm // 权限结构
	MsgStime  int64   // 最后发送时间
	MsgRtime  int64   // 最后接收时间
	MsgCtime  int64   // 最后修改时间
	MsgCbytes uint64  // 当前队列字节数
	MsgQnum   uint64  // 当前消息数量
	MsgQbytes uint64  // 队列最大字节数
	MsgLspid  int32   // 最后发送进程PID
	MsgLrpid  int32   // 最后接收进程PID
}

// MsqPerm 消息队列权限结构
type MsqPerm struct {
	Key  uint32  // IPC key
	Uid  uint32  // 拥有者UID
	Gid  uint32  // 拥有者GID
	Cuid uint32  // 创建者UID
	Cgid uint32  // 创建者GID
	Mode uint16  // 权限模式
	_    [2]byte // 填充对齐
}

// MsgCtl 控制器
type MsgCtl struct {
	qid int // 消息队列ID
}

// NewMsgCtl 创建控制器
func NewMsgCtl(qid int) *MsgCtl {
	return &MsgCtl{qid: qid}
}

// Stat 获取队列状态
func (c *MsgCtl) Stat() (*MsqidDs, error) {
	var ds MsqidDs
	_, _, errno := syscall.Syscall(
		syscall.SYS_MSGCTL,
		uintptr(c.qid),
		uintptr(IPC_STAT),
		uintptr(unsafe.Pointer(&ds)),
	)
	if errno != 0 {
		return nil, handleErrno(errno, "msg-ctl stat")
	}
	return &ds, nil
}

// Set 修改队列参数
func (c *MsgCtl) Set(ds *MsqidDs) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_MSGCTL,
		uintptr(c.qid),
		uintptr(IPC_SET),
		uintptr(unsafe.Pointer(ds)),
	)
	if errno != 0 {
		return handleErrno(errno, "msg-ctl set")
	}
	return nil
}

// Remove 删除队列
func (c *MsgCtl) Remove() error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_MSGCTL,
		uintptr(c.qid),
		uintptr(IPC_RMID),
		0,
	)
	if errno != 0 {
		return handleErrno(errno, "msg-ctl remove")
	}

	return nil
}

// 转换系统调用结构体到用户结构体
//func convertMsqidDs(cDs *syscall.MsqidDs) *MsqidDs {
//	return &MsqidDs{
//		MsgPerm: MsqPerm{
//			Key:  cDs.Perm.Key,
//			Uid:  cDs.Perm.Uid,
//			Gid:  cDs.Perm.Gid,
//			Cuid: cDs.Perm.Cuid,
//			Cgid: cDs.Perm.Cgid,
//			Mode: cDs.Perm.Mode,
//		},
//		MsgStime:  cDs.MsgStime,
//		MsgRtime:  cDs.MsgRtime,
//		MsgCtime:  cDs.MsgCtime,
//		MsgCbytes: cDs.MsgCbytes,
//		MsgQnum:   cDs.MsgQnum,
//		MsgQbytes: cDs.MsgQbytes,
//		MsgLspid:  cDs.MsgLspid,
//		MsgLrpid:  cDs.MsgLrpid,
//	}
//}

// 转换用户结构体到系统调用结构体
//func convertSyscallMsqidDs(ds *MsqidDs) *syscall.MsqidDs {
//	return &syscall.MsqidDs{
//		Perm: syscall.IpcPerm{
//			Key:  ds.MsgPerm.Key,
//			Uid:  ds.MsgPerm.Uid,
//			Gid:  ds.MsgPerm.Gid,
//			Cuid: ds.MsgPerm.Cuid,
//			Cgid: ds.MsgPerm.Cgid,
//			Mode: ds.MsgPerm.Mode,
//		},
//		MsgStime:  ds.MsgStime,
//		MsgRtime:  ds.MsgRtime,
//		MsgCtime:  ds.MsgCtime,
//		MsgCbytes: ds.MsgCbytes,
//		MsgQnum:   ds.MsgQnum,
//		MsgQbytes: ds.MsgQbytes,
//		MsgLspid:  ds.MsgLspid,
//		MsgLrpid:  ds.MsgLrpid,
//	}
//}
