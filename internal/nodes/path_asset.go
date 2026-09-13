package nodes

import (
	"github.com/yottaapp/yotta/internal/capability"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/nodecontract"
)

const PathInlineBudgetCode = "path.inline_budget_exceeded"

func defineReadPath(types []datatype.Definition, blobRead capability.Definition) (BuiltinDefinition, error) {
	inputs := []nodecontract.DataInputPort{{ID: "asset", Type: datatype.RefExpression(types[4].TypeRef()), Required: true}}
	outputs := []nodecontract.DataOutputPort{{ID: "path", Type: datatype.RefExpression(types[2].TypeRef())}}
	contract, err := nodecontract.Seal(nodecontract.Draft{
		NodeTypeID: ReadPathNodeID, Version: BuiltinNodeVersion, ConfigSchemaRoot: ReadPathNodeID + "/config", ConfigSchemaBundle: emptyConfigSchema(ReadPathNodeID + "/config"),
		Ports:     nodecontract.PortSet{DataInputs: inputs, DataOutputs: outputs, ExecInputs: signalList("in"), ExecOutputs: signalList("completed"), ErrorOutputs: signalList("failed")},
		Execution: effectExecution(ReadPathEffectID), Instruction: nodecontract.Invoke(),
		CapabilityRequirements: []capability.Requirement{requirement(blobRead, "blob-read", []string{"read-range"}, "blob-store")},
		Errors:                 []nodecontract.ErrorSpec{{Code: PathInvalidCode, Category: "navigation"}, {Code: PathInlineBudgetCode, Category: "navigation"}},
		ImplementationABI:      []nodecontract.ABIRequirement{{Kind: nodecontract.ABIBuiltin, Version: "v1"}},
		Authoring:              nodecontract.Authoring{TitleKey: "node.navigation.readPath.title", DescriptionKey: "node.navigation.readPath.description", Category: "data", Icon: "route", Tags: []string{"path", "navigation"}, Ports: dataPortHints("node.navigation.readPath", inputs, outputs, nil)},
	})
	if err != nil {
		return BuiltinDefinition{}, err
	}
	return defineBuiltin(contract, "navigation.read-path", "v1", "content-addressed-path/v1", nil)
}
