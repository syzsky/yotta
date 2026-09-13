import { describe, it, expect } from 'vitest'
import projection from '../../../../contracts/node/current/builtin-authoring'
import { resourceNodeActions } from './resourceNodeActions'
describe('resource node actions', () => {
  it('offers valid built-in nodes with a compatible resource input and completion signal', () => {
    for (const kind of ['path', 'template', 'macro', 'clip'] as const) {
      for (const action of resourceNodeActions(kind)) {
        const node = projection.body.nodes.find(
          (node) => node.nodeRef.nodeTypeId === action.nodeTypeId,
        )
        expect(node, action.nodeTypeId).toBeDefined()
        expect(node?.dataInputs.some((port) => port.id === action.portId)).toBe(true)
        expect(
          node?.signals.some(
            (signal) => signal.direction === 'output' && signal.id === action.execOutput,
          ),
        ).toBe(true)
        expect(node?.titleKey).toBe(action.titleKey)
      }
    }
  })
})
