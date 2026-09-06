package application

import (
	"github.com/yottaapp/yotta/internal/nodecontract"
	"github.com/yottaapp/yotta/internal/workflow/schema"
)

func (a *Application) NodePackageDependencies() []schema.NodePackageDependency {
	result := append([]schema.NodePackageDependency{}, a.nodePackages...)
	for i := range result {
		result[i].NodeRefs = append([]nodecontract.NodeRef(nil), result[i].NodeRefs...)
	}
	return result
}
