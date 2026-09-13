import { expect, it } from 'vitest'
import authoringDocument from '../../../../contracts/node/current/builtin-authoring'
import type { NodeProjection } from './EditorSession'
import { positionSourceStopOutputs } from './pathFollowingAuthoring'

it('preserves ordinary exec outcomes without treating error or input ports as sampler stops', () => {
  const projection = structuredClone(
    authoringDocument.body.nodes.find(
      (node) =>
        node.nodeRef.nodeTypeId === 'https://schemas.yotta.dev/nodes/navigation/follow-path',
    ),
  ) as NodeProjection
  projection.instruction = { kind: 'invoke', invoke: {} }
  projection.signals = [
    { id: 'arrived', direction: 'output', channel: 'exec' },
    { id: 'failed', direction: 'output', channel: 'error' },
    { id: 'in', direction: 'input', channel: 'exec' },
  ]
  expect(positionSourceStopOutputs(projection)).toEqual(['arrived'])
})

it.each(['follow-path', 'follow-saved-path'])(
  'stops sampling only for terminal outcomes of %s',
  (name) => {
    const projection = structuredClone(
      authoringDocument.body.nodes.find(
        (node) => node.nodeRef.nodeTypeId === `https://schemas.yotta.dev/nodes/navigation/${name}`,
      ),
    ) as NodeProjection
    const terminals = [
      'arrived',
      'timeout',
      'stuck',
      'unavailable',
      'reference-mismatch',
      'height-mismatch',
    ]
    projection.instruction = {
      kind: 'invoke',
      invoke: {
        branches: [
          { output: 'moving', coalesce: true },
          { output: 'marker' },
          { output: 'recover' },
          { output: 'future-action' },
        ],
      },
    }
    projection.signals = [...terminals, 'moving', 'marker', 'recover', 'future-action'].map(
      (id) => ({ id, direction: 'output', channel: 'exec' }),
    )
    expect(positionSourceStopOutputs(projection)).toEqual(terminals)
  },
)
