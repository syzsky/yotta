import { expect, it } from 'vitest'
import type {
  NodePackageDependency,
  YottaWorkflowSource,
} from '../../../../contracts/workflow/current/workflow-source'
import { syncNodePackageDependencies } from './nodePackageDependencies'

it('retains the exact package release when adding a plugin node, without silently upgrading existing locks', () => {
  const ref = {
    nodeTypeId: 'https://example.test/nte/read',
    version: '1.0.0',
    semanticDigest: `sha256:${'a'.repeat(64)}`,
  }
  const dependency: NodePackageDependency = {
    publisherNamespace: 'https://example.test/nte',
    packageId: 'https://example.test/nte/packages/combat/v1',
    packageVersion: '1.0.0',
    manifestDigest: `sha256:${'b'.repeat(64)}`,
    nodeRefs: [ref],
  }
  const source = {
    graphs: [{ nodes: [{ nodeRef: ref }] }],
    dependencies: [],
  } as unknown as YottaWorkflowSource
  syncNodePackageDependencies(source, [dependency])
  expect(source.dependencies).toEqual([dependency])
  syncNodePackageDependencies(source, [dependency])
  expect(source.dependencies).toHaveLength(1)
  syncNodePackageDependencies(source, [
    { ...dependency, manifestDigest: `sha256:${'c'.repeat(64)}` },
  ])
  expect(source.dependencies[0]?.manifestDigest).toBe(dependency.manifestDigest)
})
