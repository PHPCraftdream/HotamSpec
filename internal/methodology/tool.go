package methodology

import "github.com/PHPCraftdream/HotamSpecGo/internal/registry"

type Tool struct {
	Command string
	Canon   string
	Purpose string
	Run     func(args []string) error
}

var Tools = registry.New[Tool]()
