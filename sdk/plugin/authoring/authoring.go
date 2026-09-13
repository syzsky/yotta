// Package authoring exposes the host's canonical node authoring primitives to
// independently built plugins. It shares the implementation with the host.
package authoring

import (
	"github.com/yottaapp/yotta/internal/appcontrol"
	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/automation/navigation"
	"github.com/yottaapp/yotta/internal/datatype"
	"github.com/yottaapp/yotta/internal/httpegress"
	"github.com/yottaapp/yotta/internal/navigationpath"
	"github.com/yottaapp/yotta/internal/nodecatalog"
	"github.com/yottaapp/yotta/internal/nodecontract"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/pluginprotocol"
)

// WorldPosition is the host's standard, transport-independent live observation.
// Return it on a typed output, then connect a State Write node to publish it.
type WorldPosition = navigation.WorldPosition

const WorldPositionTypeID = nodes.WorldPositionTypeID

// Path is durable ordered geometry, independent of a live observation session.
type Path = navigationpath.Path
type PathPoint = navigationpath.Point
type PathReference = navigationpath.Reference

const PathTypeID = nodes.PathTypeID
const PathPointTypeID = nodes.PathPointTypeID
const PathReferenceTypeID = nodes.PathReferenceTypeID
const PathAssetTypeID = nodes.PathAssetTypeID
const PathMediaType = navigationpath.MediaType

type Catalog = nodecatalog.Snapshot
type TypeRef = datatype.TypeRef
type SchemaResource = datatype.SchemaResource
type Contract = nodecontract.Contract
type Draft = nodecontract.Draft
type ABIRequirement = nodecontract.ABIRequirement
type Authoring = nodecontract.Authoring
type ConfiguredTargetSpec = nodecontract.ConfiguredTargetSpec
type DataInputPort = nodecontract.DataInputPort
type DataOutputPort = nodecontract.DataOutputPort
type EffectID = nodecontract.EffectID
type ErrorSpec = nodecontract.ErrorSpec
type ExecutionSpec = nodecontract.ExecutionSpec
type HostFeatureRequirement = nodecontract.HostFeatureRequirement
type PortAuthoring = nodecontract.PortAuthoring
type PortSet = nodecontract.PortSet
type SignalPort = nodecontract.SignalPort

const (
	ABIProcess              = nodecontract.ABIProcess
	CacheNone               = nodecontract.CacheNone
	CancellationCooperative = nodecontract.CancellationCooperative
	EvaluationPush          = nodecontract.EvaluationPush
	ExecutionEffect         = nodecontract.ExecutionEffect
	Recorded                = nodecontract.Recorded
	RetryNever              = nodecontract.RetryNever
	TimeoutRequired         = nodecontract.TimeoutRequired
	HTTP                    = httpegress.TargetKind
	Application             = appcontrol.TargetKind
	ProcessIsolation        = pluginprotocol.ProcessIsolationHostFeatureID
)

var Seal = nodecontract.Seal
var Invoke = nodecontract.Invoke
var RefExpression = datatype.RefExpression
var RefResolvedType = datatype.RefResolvedType
var SealInlineJSON = datatype.SealInlineJSON
var Marshal = artifact.Marshal

// Builtins resolves the exact built-in types for this SDK release.
func Builtins() (Catalog, error) {
	b, err := nodes.Build()
	return b.Catalog, err
}
