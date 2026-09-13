import type { NodeProjection } from './EditorSession'

export function isPathFollower(nodeTypeId: string): boolean {
  return ['follow-path', 'follow-saved-path'].some(
    (name) => nodeTypeId === `https://schemas.yotta.dev/nodes/navigation/${name}`,
  )
}

export function positionSourceStopOutputs(projection: NodeProjection): string[] {
  const branches = new Set(
    projection.instruction.kind === 'invoke'
      ? (projection.instruction.invoke.branches ?? []).map((branch) => branch.output)
      : [],
  )
  return projection.signals
    .filter(
      (signal) =>
        signal.direction === 'output' && signal.channel === 'exec' && !branches.has(signal.id),
    )
    .map((signal) => signal.id)
}
