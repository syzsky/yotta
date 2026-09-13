import type { InjectionKey } from 'vue'
import type { EditorSession, EditorCommand, Node, Edge } from './EditorSession'
import { positionSourceStopOutputs } from './pathFollowingAuthoring'
export const POSITION_SOURCE_AUTHORING: InjectionKey<{
  variables: () => Array<{ label: string; value: string }>
  connect: (nodeId: string, slot: string) => string
}> = Symbol('position-source-authoring')
export const POSITION_STRING_TYPE = 'https://schemas.yotta.dev/types/core/string/v1'
export function connectPositionSource(
  session: EditorSession,
  nodeId: string,
  slot: string,
): string {
  const graph = session.currentGraph
  const follower = graph?.nodes.find((node) => node.id === nodeId)
  if (!graph || !follower || !slot) throw new Error('Position source target is unavailable')
  const followerProjection = session.nodeProjection(follower.nodeRef.nodeTypeId)
  if (!followerProjection) throw new Error('Position source target is unavailable')
  const prefix = 'https://schemas.yotta.dev/nodes/'
  const existing = follower.config['position-variable']
  if (
    typeof existing === 'string' &&
    session.source?.variables.some((variable) => variable.name === existing)
  ) {
    const writers = graph.nodes.filter(
      (node) =>
        node.nodeRef.nodeTypeId === prefix + 'state/write' && node.config.variable === existing,
    )
    const reusable = writers.some((writer) =>
      graph.edges.some((edge) => {
        if (
          edge.channel !== 'data' ||
          edge.to.nodeId !== writer.id ||
          edge.to.portId !== 'value' ||
          edge.from.portId !== 'body'
        )
          return false
        const getter = graph.nodes.find((node) => node.id === edge.from.nodeId)
        return (
          getter?.nodeRef.nodeTypeId === prefix + 'network/http-get' &&
          getter.config.slot === slot &&
          getter.bindings.path?.kind === 'value' &&
          getter.bindings.path.value === '/v1/position-source/sample'
        )
      }),
    )
    if (reusable) return existing
  }

  const get = session.nodeProjection(prefix + 'network/http-get')
  const stringType = get?.dataOutputs.find((port) => port.id === 'body')?.type.expression
  if (!stringType) throw new Error('Position source sampling nodes are unavailable')
  let name = 'position-source'
  for (
    let n = 2;
    session.source?.variables.some(
      (variable) => variable.name === name || variable.name === `${name}.active`,
    );
    n++
  )
    name = `position-source-${n}`
  const activeName = `${name}.active`
  const booleanType = session
    .nodeProjection(prefix + 'control/branch')
    ?.dataInputs.find((port) => port.id === 'condition')?.type.expression
  if (!booleanType) throw new Error('Position source condition type is unavailable')
  const defs = [
    ['event/run-started', {}, {}],
    ['control/periodic', {}, { 'interval-milliseconds': { kind: 'value', value: 100 } }],
    [
      'network/http-get',
      { slot },
      { path: { kind: 'value', value: '/v1/position-source/sample' } },
    ],
    ['state/write', { variable: name }, {}],
    ['state/read', { variable: activeName }, {}],
    ['control/branch', {}, {}],
    ['state/write', { variable: activeName }, { value: { kind: 'value', value: false } }],
  ] as const
  const nodes: Node[] = defs.map(([type, config, bindings], index) => {
    const projection = session.nodeProjection(prefix + type)
    if (!projection) throw new Error('Position source sampling node is unavailable')
    return {
      id: `node_${crypto.randomUUID()}`,
      nodeRef: structuredClone(projection.nodeRef),
      config: structuredClone(config),
      bindings: structuredClone(bindings),
      position: { x: follower.position.x + index * 290, y: follower.position.y + 420 },
    }
  })
  const edge = (
    a: number,
    out: string,
    b: number,
    input: string,
    channel: 'exec' | 'data' = 'exec',
  ): Edge => ({
    channel,
    from: { nodeId: nodes[a]!.id, portId: out },
    to: { nodeId: nodes[b]!.id, portId: input },
  })
  const commands: EditorCommand[] = [
    { kind: 'add-state-variable', name, type: structuredClone(stringType), defaultValue: '' },
    {
      kind: 'add-state-variable',
      name: activeName,
      type: structuredClone(booleanType),
      defaultValue: true,
    },
    {
      kind: 'insert-node-selection',
      nodes,
      calls: [],
      annotations: [],
      edges: [
        edge(0, 'started', 1, 'in'),
        edge(1, 'tick', 5, 'in'),
        edge(4, 'result', 5, 'condition', 'data'),
        edge(5, 'true', 2, 'in'),
        edge(5, 'false', 1, 'stop'),
        edge(2, 'completed', 3, 'in'),
        edge(2, 'body', 3, 'value', 'data'),
        ...positionSourceStopOutputs(followerProjection).map((output) => ({
          channel: 'exec' as const,
          from: { nodeId, portId: output },
          to: { nodeId: nodes[6]!.id, portId: 'in' },
        })),
      ],
    },
    { kind: 'set-config', nodeId, fieldId: 'position-variable', value: name },
  ]
  session.applyBatch(commands)
  return name
}
