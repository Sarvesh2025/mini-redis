package core

type ClientContext struct {
	InMulti     bool
	TxQueue     []*RedisCmd
	WatchedKeys map[string]uint64
}

func NewClientContext() *ClientContext {
	return &ClientContext{}
}

func (c *ClientContext) Reset() {
	c.InMulti = false
	c.TxQueue = nil
	c.WatchedKeys = nil
}

func (c *ClientContext) QueueCmd(cmd *RedisCmd) {
	c.TxQueue = append(c.TxQueue, cmd)
}
