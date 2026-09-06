package authoring

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/nodecontract"
	"github.com/yottaapp/yotta/internal/workflow/schema"
)

const (
	aiExtractNodeTypeID        = "https://schemas.yotta.dev/nodes/ai/extract"
	aiGenerateNodeTypeID       = "https://schemas.yotta.dev/nodes/ai/generate"
	clickTemplateNodeTypeID    = "https://schemas.yotta.dev/nodes/automation/click-template"
	movePointerNodeTypeID      = "https://schemas.yotta.dev/nodes/automation/move-pointer"
	playInputClipNodeTypeID    = "https://schemas.yotta.dev/nodes/automation/play-input-clip"
	waitTemplateNodeTypeID     = "https://schemas.yotta.dev/nodes/automation/wait-template"
	waitTemplateGoneNodeTypeID = "https://schemas.yotta.dev/nodes/automation/wait-template-gone"
	waitChangeNodeTypeID       = "https://schemas.yotta.dev/nodes/automation/wait-change"
	waitStableNodeTypeID       = "https://schemas.yotta.dev/nodes/automation/wait-stable"
	waitWindowNodeTypeID       = "https://schemas.yotta.dev/nodes/automation/wait-window"
	waitWindowGoneNodeTypeID   = "https://schemas.yotta.dev/nodes/automation/wait-window-gone"
	retryNodeTypeID            = "https://schemas.yotta.dev/nodes/control/retry"
	currentNodeContractV100    = "1.0.0"
	currentNodeContractV110    = "1.1.0"
	legacyNodeContractV100     = "1.0.0"
	movePointerLegacyDuration  = "300"
	movePointerLegacyMotion    = `"linear"`
)

type nodeContractMigrationKind uint8

const (
	nodeContractMigrationShapeCompatible nodeContractMigrationKind = iota
	nodeContractMigrationRetractPlayInputClipScale
	nodeContractMigrationPreserveMovePointerDefaults
)

type nodeContractMigration struct {
	nodeTypeID string
	from       string
	fromDigest string
	to         string
	toDigest   string
	kind       nodeContractMigrationKind
}

// nodeContractMigrations is intentionally an exact, finite registry. A new
// digest or non-adjacent version is unsupported until its migration is added
// with a frozen regression fixture.
var nodeContractMigrations = [...]nodeContractMigration{
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/button", from: "1.1.0", fromDigest: "sha256:7b3469750e612338155f638ce8e0f1f96e88e8bdb9893572594145b433d3ac09", to: "1.2.0", toDigest: "sha256:9705ed62741914ddb8af604537d6e16710456de5d77edf87ce3a3f4a5faf13ca", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/create", from: "1.1.0", fromDigest: "sha256:d97de952f7f686b63d76eb254e84bb7763b9a2880ea38d67fc9df0685fde5909", to: "1.2.0", toDigest: "sha256:5f98f0e628501bf8f6f3b8ea41a9e43bb557b65d817cfcb6de6f2b6279084ae0", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/end", from: "1.1.0", fromDigest: "sha256:76ab41d98ba7e56deb1cc1fed0786e6aba8d00940a53ce8caed4295e523ab089", to: "1.2.0", toDigest: "sha256:0c159e37a3ffe713c8626ff6be11238a574607d07c9bdca7a7d1d875117cfec4", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/input", from: "1.1.0", fromDigest: "sha256:4baca1a68a3c6f7035072304c468c4778cedd2fd0dc5395653e39e40ee1f84ec", to: "1.2.0", toDigest: "sha256:c9bf557dc30136926b50ad0de3d475e19977e19ca4e0434d441421a2f6e3a4a5", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/log", from: "1.1.0", fromDigest: "sha256:9fad6cedbea9512d77c60091abf67dfeda83a461172e9da78db8e72e36dd3ba0", to: "1.2.0", toDigest: "sha256:4eeb715690f26f40549c9c4f792ac411bc4ac3a81c0f5bd23679bb2fcef817eb", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/number", from: "1.1.0", fromDigest: "sha256:321444a78613136afdee82b30cf539b52038fed162777d8c0ac1119a3c1dcbbc", to: "1.2.0", toDigest: "sha256:629d4391630bf3bd96c5f7ff65532c8b07f50ddafa0f516e9be9e1127ff07576", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/read-number", from: "1.1.0", fromDigest: "sha256:f643f17f898ab07f3c56d740b22114693edcb2257b3d1ba098416fe1df7b40be", to: "1.2.0", toDigest: "sha256:1dd7d0e1f3ca2d235233a1600526e5059f5a46b6e857bdd25cc959e1205aa7da", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/read-text", from: "1.1.0", fromDigest: "sha256:608e4c061168213ec56a5350dc2aab0b8df1b68fb5e1761b1a0176e80e5b09b5", to: "1.2.0", toDigest: "sha256:35dd64426872c58cea85ddf07ab0460e2901a7c6e1a3d849b5665d3f234bbf85", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/read-toggle", from: "1.1.0", fromDigest: "sha256:c06afe2156ad91d4500db08f35290604ef5e1ea7f9998033d1f4fd14da1dd761", to: "1.2.0", toDigest: "sha256:28d2085b47d22597ec7c7d112bf1bb99a906beaa5eb60977240d4b8b367f2caf", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/select", from: "1.1.0", fromDigest: "sha256:c37b7d9a04af72d9a97aecf41f78eefb4ba5768faa95e662421843f76d95aabe", to: "1.2.0", toDigest: "sha256:e1fa5391c34081483cae2a556ceb00cd65dd55924165e9a3d1cb979611910e57", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/status", from: "1.1.0", fromDigest: "sha256:9c5350638c3538d76b0857f98b9498f99979089475b45d0d1c1b09a11487413b", to: "1.2.0", toDigest: "sha256:85450912ea6a433f7b3446ef70fab02dbe567745772d0394990ca086100d8d38", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/text", from: "1.1.0", fromDigest: "sha256:6f2f3d34e9436090602aea49e9724950d1c303a096d5310a48475794ff33a6cc", to: "1.2.0", toDigest: "sha256:219acadde0a491fbb3b15e5a09c3ea4ebcdccb67c9076220b431b70004bad7a4", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/toggle", from: "1.1.0", fromDigest: "sha256:32df40b5b85849de2d10c355c5d4aee7ea4406cd09c04c816b9ee1c2fdc56908", to: "1.2.0", toDigest: "sha256:d920a1df1eb389f2534309195d962ceac2c17939c52c6043aa82c526ef0c01f9", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/wait", from: "1.1.0", fromDigest: "sha256:7e78a5035fd2f8b4ff1965cbf6ecb3fc3fd8d30add1611ee5fbdb6b13c2993a2", to: "1.2.0", toDigest: "sha256:5dda0b76e0436f36fcdd61fe715d4b8a992737106c79edec1070305f00080f10", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/write-number", from: "1.1.0", fromDigest: "sha256:1ed99f60fd3406442436120d7555ee8dec1babcd408be7e9e126b666bbe42bb6", to: "1.2.0", toDigest: "sha256:3a7cc17dcbcfbd8fae0db2b999da0d776ebb4a5ac82bcb03c73b8738c4cb6667", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/write-text", from: "1.1.0", fromDigest: "sha256:9d53cf880c5f055e02aef47a67f60968c8e0c8c8b429026303367750e4b46eb3", to: "1.2.0", toDigest: "sha256:91be54690b42cbca14ab36f8b14fc886425c305d49f483090371338da532350c", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/write-toggle", from: "1.1.0", fromDigest: "sha256:cebe57a893f033add3cea77efbdc9e2e04fc783da98db87dcc462a4225450c1c", to: "1.2.0", toDigest: "sha256:ffe505f255e995b237a12e610225b5e2a97eee4f2222925c836ab4cfc90a323e", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/button", from: "1.0.0", fromDigest: "sha256:67f953b7ee032963a0d92c6c88e704c3d9b7a86beb91570dd5c1d4897d3a03f9", to: "1.1.0", toDigest: "sha256:7b3469750e612338155f638ce8e0f1f96e88e8bdb9893572594145b433d3ac09", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/create", from: "1.0.0", fromDigest: "sha256:ef59601bd08d6ae0f9b26a88a1d17fcac6550baeb0be1e48c4d379df213a19b0", to: "1.1.0", toDigest: "sha256:d97de952f7f686b63d76eb254e84bb7763b9a2880ea38d67fc9df0685fde5909", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/end", from: "1.0.0", fromDigest: "sha256:f565814fe46131a4cd7038907a63598f745d7975f8625733f52f2ba1079faf94", to: "1.1.0", toDigest: "sha256:76ab41d98ba7e56deb1cc1fed0786e6aba8d00940a53ce8caed4295e523ab089", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/input", from: "1.0.0", fromDigest: "sha256:594e7fe8d5099a9f378cf2bf7d35765cb9c27f4d62928c33715171e21efaec74", to: "1.1.0", toDigest: "sha256:4baca1a68a3c6f7035072304c468c4778cedd2fd0dc5395653e39e40ee1f84ec", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/log", from: "1.0.0", fromDigest: "sha256:c60ffa077ddfd9ebcb3633e58da3bdd7d7762d7a3a910579ac281fdd53d95fd6", to: "1.1.0", toDigest: "sha256:9fad6cedbea9512d77c60091abf67dfeda83a461172e9da78db8e72e36dd3ba0", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/number", from: "1.0.0", fromDigest: "sha256:7cc9533117a583e7386195f20f4efdf873cf4affe0eebde5b287530a1546c5d3", to: "1.1.0", toDigest: "sha256:321444a78613136afdee82b30cf539b52038fed162777d8c0ac1119a3c1dcbbc", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/read-number", from: "1.0.0", fromDigest: "sha256:d86eba0796f349ca5c8be5cfd814ecfa8138f4ce74d986b12cfc9f7f4e9bb89b", to: "1.1.0", toDigest: "sha256:f643f17f898ab07f3c56d740b22114693edcb2257b3d1ba098416fe1df7b40be", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/read-text", from: "1.0.0", fromDigest: "sha256:106c6329340144beb836a0fcb4fa1708897d0ed47e9304f0c6505b27ec9e217a", to: "1.1.0", toDigest: "sha256:608e4c061168213ec56a5350dc2aab0b8df1b68fb5e1761b1a0176e80e5b09b5", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/read-toggle", from: "1.0.0", fromDigest: "sha256:4b69b2ecc5b58856d1955022aa5655498f8663e7f0d134699053e740be4750dc", to: "1.1.0", toDigest: "sha256:c06afe2156ad91d4500db08f35290604ef5e1ea7f9998033d1f4fd14da1dd761", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/select", from: "1.0.0", fromDigest: "sha256:b75ac38828f8149784453065a20c983af886c530b6a9b41bbe43309f07c2fd92", to: "1.1.0", toDigest: "sha256:c37b7d9a04af72d9a97aecf41f78eefb4ba5768faa95e662421843f76d95aabe", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/status", from: "1.0.0", fromDigest: "sha256:8d1a10585d397077870c5da8309c7bf5d30caf9d763cf140cbc0a3c24da4a184", to: "1.1.0", toDigest: "sha256:9c5350638c3538d76b0857f98b9498f99979089475b45d0d1c1b09a11487413b", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/text", from: "1.0.0", fromDigest: "sha256:5c41391f034313c260f534b4738945bf9d0cfb43982c4513fb2e77e2e41c901e", to: "1.1.0", toDigest: "sha256:6f2f3d34e9436090602aea49e9724950d1c303a096d5310a48475794ff33a6cc", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/toggle", from: "1.0.0", fromDigest: "sha256:2bd2212461d7f1ead1344ff5a23dd0ab6c3942bfb775af1a7a973c03b6eabf99", to: "1.1.0", toDigest: "sha256:32df40b5b85849de2d10c355c5d4aee7ea4406cd09c04c816b9ee1c2fdc56908", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/wait", from: "1.0.0", fromDigest: "sha256:8257c4a268edaf4d5b67fb09d898deaefb86a7c8b45ac21874090bf9c92ad598", to: "1.1.0", toDigest: "sha256:7e78a5035fd2f8b4ff1965cbf6ecb3fc3fd8d30add1611ee5fbdb6b13c2993a2", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/write-number", from: "1.0.0", fromDigest: "sha256:f2da05845e22c9c844fefe8b9b3ac960095d23256b003fd7cb8163013ee58c37", to: "1.1.0", toDigest: "sha256:1ed99f60fd3406442436120d7555ee8dec1babcd408be7e9e126b666bbe42bb6", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/write-text", from: "1.0.0", fromDigest: "sha256:6d4082d3074136d9f66912d1bce551398dc6852301adb8abb4053726ba4cee04", to: "1.1.0", toDigest: "sha256:9d53cf880c5f055e02aef47a67f60968c8e0c8c8b429026303367750e4b46eb3", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: "https://schemas.yotta.dev/nodes/panel/write-toggle", from: "1.0.0", fromDigest: "sha256:1b175c1bcc6114886106240ab2d7b7d55b67d03b764561a7f17562b5b5dd859e", to: "1.1.0", toDigest: "sha256:cebe57a893f033add3cea77efbdc9e2e04fc783da98db87dcc462a4225450c1c", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: waitChangeNodeTypeID, from: legacyNodeContractV100, fromDigest: "sha256:432e584966b1e6b8b34eb79ac1c53106abba50f71a789cc3cce2b0d94052843a", to: currentNodeContractV110, toDigest: "sha256:1ccfedf422c5e9245916a0c9d0f86710f9fd6046b6f8fd85f0c2df78f08c399a", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: waitStableNodeTypeID, from: legacyNodeContractV100, fromDigest: "sha256:dfc129479fa4c8d2c63411070d6fc54be971e43488330383a3c5f7a5ad76087a", to: currentNodeContractV110, toDigest: "sha256:cafd06f6193933bac354ee995f9766d5bf7ff4fb3e55de33e5e3c7dbfde09cf6", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: waitWindowNodeTypeID, from: legacyNodeContractV100, fromDigest: "sha256:8ff0677fa53934626b1c2f01f620bd7385073273aac27553d320551b7807b5af", to: currentNodeContractV110, toDigest: "sha256:1662e4ee4b3d8b49d127dd3c25f4d82e218ae2d99d5f3ca45eb5522282b2d895", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: waitWindowGoneNodeTypeID, from: legacyNodeContractV100, fromDigest: "sha256:29a57cf1301409914bb6948f2f257ee2ce9cb983b2f4e8337b38761a53caafb2", to: currentNodeContractV110, toDigest: "sha256:91b3e42d77c56a1a5d0e4f2c350df9c11bc70b59ec67ab1487cd38ec8549bd16", kind: nodeContractMigrationShapeCompatible},
	{nodeTypeID: retryNodeTypeID, from: legacyNodeContractV100, fromDigest: "sha256:48867401d7ca214276ec32171de668e4ffbd7751663c6859c6afbbe0df41ffa2", to: currentNodeContractV110, toDigest: "sha256:61043a9e31514bb01c165bb5d64693b170818448c5aca266699892debaaf4ff7", kind: nodeContractMigrationShapeCompatible},
	{
		nodeTypeID: clickTemplateNodeTypeID,
		from:       legacyNodeContractV100,
		fromDigest: "sha256:370ee214c1f0e99d149a5f709019e74480f9054442e058ae6349054997f72c1d",
		to:         currentNodeContractV100,
		toDigest:   "sha256:16babb2401a04127a949e0b855179ce03233a89dc79e63e93f8643e416e208b6",
		kind:       nodeContractMigrationShapeCompatible,
	},
	{
		nodeTypeID: waitTemplateNodeTypeID,
		from:       legacyNodeContractV100,
		fromDigest: "sha256:cd4f4177c886c36ba2ede3634b67b87c4d22597fa86e21269cae9bc38cc2066e",
		to:         currentNodeContractV100,
		toDigest:   "sha256:2c28ed5b6ff228dd5d5e083b368f59a73a92a265364df07f8bdc355e9c282435",
		kind:       nodeContractMigrationShapeCompatible,
	},
	{
		nodeTypeID: waitTemplateGoneNodeTypeID,
		from:       legacyNodeContractV100,
		fromDigest: "sha256:28eaccad35ee50f132dce3e2a45a85ad09d2c59d99c48dcdd42f9f7d3fb80253",
		to:         currentNodeContractV100,
		toDigest:   "sha256:a5366ec1e69d656de1b3c714e432420c0a1d65b83c01ff3fd235c50834a3d62b",
		kind:       nodeContractMigrationShapeCompatible,
	},
	{
		nodeTypeID: movePointerNodeTypeID,
		from:       legacyNodeContractV100,
		fromDigest: "sha256:2bf1f8059f1269e407d2aedf4f717cc6c0b860eb46b92abd1794a3aa3bf559af",
		to:         currentNodeContractV110,
		toDigest:   "sha256:0e632e15564076e0292a3da0672c7e7a5cd852d19fd9603acab0c1fd568aec2a",
		kind:       nodeContractMigrationPreserveMovePointerDefaults,
	},
	{
		nodeTypeID: playInputClipNodeTypeID,
		from:       legacyNodeContractV100,
		fromDigest: "sha256:ff7ea9d0b2ca91cb2062cff30dd5ca8575555ec5363b4c76e746925ee6ae027b",
		to:         currentNodeContractV110,
		toDigest:   "sha256:abc200829a50376c1ad914f0ae25b1dea61a874a824a49f850a5ed4839db16a8",
		kind:       nodeContractMigrationRetractPlayInputClipScale,
	},
	{
		nodeTypeID: playInputClipNodeTypeID,
		from:       legacyNodeContractV100,
		fromDigest: "sha256:5c353fb0725ca6a841a7ef5e9adcca12bb10e2d6362fed4d7d38449a58608e02",
		to:         currentNodeContractV110,
		toDigest:   "sha256:abc200829a50376c1ad914f0ae25b1dea61a874a824a49f850a5ed4839db16a8",
		kind:       nodeContractMigrationShapeCompatible,
	},
	{
		nodeTypeID: playInputClipNodeTypeID,
		from:       legacyNodeContractV100,
		fromDigest: "sha256:bab93b5e1f655e3f5e23c254139b92a23b048c93d3d212ff7f32d2dd009e0d75",
		to:         currentNodeContractV110,
		toDigest:   "sha256:abc200829a50376c1ad914f0ae25b1dea61a874a824a49f850a5ed4839db16a8",
		kind:       nodeContractMigrationShapeCompatible,
	},
	{
		nodeTypeID: playInputClipNodeTypeID,
		from:       legacyNodeContractV100,
		fromDigest: "sha256:0d6e75a9c06ef29cc8bdeb79f7f79f420461d9abb05ad4a9196d74324c7d2545",
		to:         currentNodeContractV110,
		toDigest:   "sha256:abc200829a50376c1ad914f0ae25b1dea61a874a824a49f850a5ed4839db16a8",
		kind:       nodeContractMigrationShapeCompatible,
	},
	{
		nodeTypeID: aiExtractNodeTypeID,
		from:       legacyNodeContractV100,
		fromDigest: "sha256:dbcb528cb623272c3a7544c1a2ff6ed2e77c14dba1d795b6fea9511f87d99646",
		to:         currentNodeContractV110,
		toDigest:   "sha256:05a063bb119608c66afac903198c60ac8763dafd732e2c4af550693a998d70af",
		kind:       nodeContractMigrationShapeCompatible,
	},
	{
		nodeTypeID: aiExtractNodeTypeID,
		from:       legacyNodeContractV100,
		fromDigest: "sha256:eee97d21d98e56ffec8d7e1cf4cd6b4ccc667394e5419f0b3ad0eab465d79a85",
		to:         currentNodeContractV110,
		toDigest:   "sha256:05a063bb119608c66afac903198c60ac8763dafd732e2c4af550693a998d70af",
		kind:       nodeContractMigrationShapeCompatible,
	},
	{
		nodeTypeID: aiGenerateNodeTypeID,
		from:       legacyNodeContractV100,
		fromDigest: "sha256:28e1267b308079ea892e23b3e89c0d97c1a6ddd81891cb42eb4157ee9a2af30a",
		to:         currentNodeContractV110,
		toDigest:   "sha256:00f2342d44deca9db66ab5d43c80d2484216a207bc7c8be6edfe088cfead9fc1",
		kind:       nodeContractMigrationShapeCompatible,
	},
}

func admittedNodeContractUpgradePath(from, to nodecontract.NodeRef) ([]nodeContractMigration, bool) {
	return nodeContractUpgradePath(nodeContractMigrations[:], from, to)
}

func nodeContractUpgradePath(
	registry []nodeContractMigration,
	from, to nodecontract.NodeRef,
) ([]nodeContractMigration, bool) {
	if from == to {
		return []nodeContractMigration{}, true
	}
	if from.NodeTypeID == "" || from.NodeTypeID != to.NodeTypeID {
		return nil, false
	}
	path := make([]nodeContractMigration, 0, len(registry))
	seen := make(map[nodecontract.NodeRef]struct{}, len(registry))
	cursor := from
	for cursor != to {
		if _, cycle := seen[cursor]; cycle {
			return nil, false
		}
		seen[cursor] = struct{}{}
		var next nodeContractMigration
		matches := 0
		for _, migration := range registry {
			if cursor == migrationFromRef(migration) {
				next = migration
				matches++
			}
		}
		if matches != 1 {
			return nil, false
		}
		path = append(path, next)
		cursor = migrationToRef(next)
		if len(path) > len(registry) {
			return nil, false
		}
	}
	return path, true
}

func migrationFromRef(migration nodeContractMigration) nodecontract.NodeRef {
	return nodecontract.NodeRef{
		NodeTypeID:     migration.nodeTypeID,
		Version:        migration.from,
		SemanticDigest: nodecontractDigest(migration.fromDigest),
	}
}

func migrationToRef(migration nodeContractMigration) nodecontract.NodeRef {
	return nodecontract.NodeRef{
		NodeTypeID:     migration.nodeTypeID,
		Version:        migration.to,
		SemanticDigest: nodecontractDigest(migration.toDigest),
	}
}

func nodecontractDigest(value string) artifact.Digest {
	return artifact.Digest(value)
}

// ValidateReleasedNodeContracts is the release-gate interface for built-in
// NodeRefs. A released type must still exist; a changed semantic digest must
// use a strictly newer stable node version and have a complete adjacent
// migration path to the current Catalog ref.
func ValidateReleasedNodeContracts(released, current []nodecontract.NodeRef) error {
	currentByType := make(map[string]nodecontract.NodeRef, len(current))
	for _, ref := range current {
		if err := validateReleaseNodeRef(ref); err != nil {
			return fmt.Errorf("current node contract: %w", err)
		}
		if _, duplicate := currentByType[ref.NodeTypeID]; duplicate {
			return fmt.Errorf("current node contract %q is duplicated", ref.NodeTypeID)
		}
		currentByType[ref.NodeTypeID] = ref
	}
	releasedByType := make(map[string]struct{}, len(released))
	for _, previous := range released {
		if err := validateReleaseNodeRef(previous); err != nil {
			return fmt.Errorf("released node contract: %w", err)
		}
		if _, duplicate := releasedByType[previous.NodeTypeID]; duplicate {
			return fmt.Errorf("released node contract %q is duplicated", previous.NodeTypeID)
		}
		releasedByType[previous.NodeTypeID] = struct{}{}
		next, found := currentByType[previous.NodeTypeID]
		if !found {
			return fmt.Errorf("released node contract %q was removed without a supported replacement", previous.NodeTypeID)
		}
		if previous == next {
			continue
		}
		if compareStableNodeVersion(next.Version, previous.Version) <= 0 {
			return fmt.Errorf(
				"released node contract %q changed semantic digest without a newer stable version (%s -> %s)",
				previous.NodeTypeID, previous.Version, next.Version,
			)
		}
		if _, ok := admittedNodeContractUpgradePath(previous, next); !ok {
			return fmt.Errorf(
				"released node contract %q has no migration path from %s/%s to %s/%s",
				previous.NodeTypeID, previous.Version, previous.SemanticDigest, next.Version, next.SemanticDigest,
			)
		}
	}
	return nil
}

func validateReleaseNodeRef(ref nodecontract.NodeRef) error {
	if strings.TrimSpace(ref.NodeTypeID) == "" || !ref.SemanticDigest.Valid() {
		return errors.New("node ref identity is invalid")
	}
	if _, ok := stableNodeVersion(ref.Version); !ok {
		return fmt.Errorf("node %q version %q is not stable numeric SemVer", ref.NodeTypeID, ref.Version)
	}
	return nil
}

func compareStableNodeVersion(left, right string) int {
	a, aOK := stableNodeVersion(left)
	b, bOK := stableNodeVersion(right)
	if !aOK || !bOK {
		return 0
	}
	for index := range a {
		if a[index] < b[index] {
			return -1
		}
		if a[index] > b[index] {
			return 1
		}
	}
	return 0
}

func stableNodeVersion(value string) ([3]uint64, bool) {
	var result [3]uint64
	parts := strings.Split(value, ".")
	if len(parts) != len(result) {
		return result, false
	}
	for index, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
			return result, false
		}
		parsed, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return result, false
		}
		result[index] = parsed
	}
	return result, true
}

func prepareAdmittedNodeContractUpgrade(migration nodeContractMigration, graph *schema.Graph, node *schema.Node) error {
	switch migration.kind {
	case nodeContractMigrationShapeCompatible:
		return nil
	case nodeContractMigrationRetractPlayInputClipScale:
		delete(node.Bindings, "turn-scale")
		return nil
	case nodeContractMigrationPreserveMovePointerDefaults:
		if node.Bindings == nil {
			node.Bindings = make(map[string]schema.InputBinding)
		}
		materializeLegacyDefault(graph, node, "duration", json.RawMessage(movePointerLegacyDuration))
		materializeLegacyDefault(graph, node, "motion", json.RawMessage(movePointerLegacyMotion))
		return nil
	default:
		return fmt.Errorf("node contract migration kind %d is unsupported", migration.kind)
	}
}

func materializeLegacyDefault(graph *schema.Graph, node *schema.Node, portID string, value json.RawMessage) {
	binding, bound := node.Bindings[portID]
	if bound && binding.Kind != schema.BindingDefault {
		return
	}
	if !bound && hasIncomingData(graph, node.ID, portID) {
		return
	}
	node.Bindings[portID] = schema.InputBinding{Kind: schema.BindingValue, Value: value}
}

func hasIncomingData(graph *schema.Graph, nodeID, portID string) bool {
	for _, edge := range graph.Edges {
		if edge.Channel == schema.EdgeData && edge.To.NodeID == nodeID && edge.To.PortID == portID {
			return true
		}
	}
	for _, input := range graph.Inputs {
		if input.NodeID == nodeID && input.PortID == portID {
			return true
		}
	}
	return false
}
