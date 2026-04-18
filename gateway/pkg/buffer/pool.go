package buffer

import (
    "sync"
)

type Pool struct {
    small  sync.Pool
    medium sync.Pool
    large  sync.Pool
}

func NewPool() *Pool {
    return &Pool{
        small: sync.Pool{
            New: func() interface{} {
                return make([]byte, 256)
            },
        },
        medium: sync.Pool{
            New: func() interface{} {
                return make([]byte, 4096)
            },
        },
        large: sync.Pool{
            New: func() interface{} {
                return make([]byte, 32768)
            },
        },
    }
}

func (p *Pool) Get(size int) []byte {
    switch {
    case size <= 256:
        return p.small.Get().([]byte)[:0]
    case size <= 4096:
        return p.medium.Get().([]byte)[:0]
    default:
        return p.large.Get().([]byte)[:0]
    }
}

func (p *Pool) Put(buf []byte) {
    switch cap(buf) {
    case 256:
        buf = buf[:256]
        p.small.Put(buf)
    case 4096:
        buf = buf[:4096]
        p.medium.Put(buf)
    case 32768:
        buf = buf[:32768]
        p.large.Put(buf)
    }
}