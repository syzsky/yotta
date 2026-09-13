export type ResourceActionKind = 'path' | 'template' | 'image' | 'macro' | 'clip' | 'input-clip'
export interface ResourceNodeAction {
  nodeTypeId: string
  titleKey: string
  portId: string
  execOutput: string
}
const action = (
  path: string,
  titleKey: string,
  portId: string,
  execOutput = 'completed',
): ResourceNodeAction => ({
  nodeTypeId: `https://schemas.yotta.dev/nodes/${path}`,
  titleKey,
  portId,
  execOutput,
})
export function resourceNodeActions(kind: ResourceActionKind): ResourceNodeAction[] {
  if (kind === 'path')
    return [
      action(
        'navigation/follow-saved-path',
        'node.navigation.followSavedPath.title',
        'asset',
        'arrived',
      ),
      action('navigation/read-path', 'node.navigation.readPath.title', 'asset'),
    ]
  if (kind === 'macro')
    return [action('automation/play-macro', 'node.automation.playMacro.title', 'macro')]
  if (kind === 'clip' || kind === 'input-clip')
    return [action('automation/play-input-clip', 'node.automation.playInputClip.title', 'clip')]
  return [
    action('automation/click-template', 'node.automation.clickTemplate.title', 'template'),
    action('automation/wait-template', 'node.automation.waitTemplate.title', 'template', 'found'),
    action(
      'automation/wait-template-gone',
      'node.automation.waitTemplateGone.title',
      'template',
      'gone',
    ),
  ]
}
