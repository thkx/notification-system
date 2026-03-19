package di

import (
	"fmt"
	"sync"
)

// Container 依赖注入容器
type Container struct {
	services map[string]interface{}
	mutex    sync.RWMutex
}

// NewContainer 创建一个新的容器实例
func NewContainer() *Container {
	return &Container{
		services: make(map[string]interface{}),
	}
}

// Register 注册服务（向后兼容）
func (c *Container) Register(name string, service interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.services[name] = service
}

// Get 获取服务（向后兼容）
func (c *Container) Get(name string) interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.services[name]
}

// GetOrPanic 获取服务，不存在则panic（向后兼容）
func (c *Container) GetOrPanic(name string) interface{} {
	service := c.Get(name)
	if service == nil {
		panic("service not found: " + name)
	}
	return service
}

// ============ 泛型支持（Go 1.18+）============

// Set 以泛型方式注册服务（编译时类型检查）
// @param c 容器
// @param name 服务名称
// @param service 服务实例
func Set[T any](c *Container, name string, service T) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.services[name] = service
}

// Get 以泛型方式获取服务（编译时类型检查）
// @param c 容器
// @param name 服务名称
// @return 服务实例和可能的错误
func Get[T any](c *Container, name string) (T, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	service, exists := c.services[name]
	var zero T

	if !exists {
		return zero, fmt.Errorf("service not found: %s", name)
	}

	typed, ok := service.(T)
	if !ok {
		return zero, fmt.Errorf("type mismatch for service %s: expected %T, got %T", name, zero, service)
	}

	return typed, nil
}

// MustGet 以泛型方式获取服务，不存在或类型不匹配则panic
// @param c 容器
// @param name 服务名称
// @return 服务实例
func MustGet[T any](c *Container, name string) T {
	service, err := Get[T](c, name)
	if err != nil {
		panic(err)
	}
	return service
}
