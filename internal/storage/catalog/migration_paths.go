package catalog

import "strings"

// Rebuild the constrained asset table in the migration transaction. Child rows
// are copied before dropping their tables so ON DELETE CASCADE loses no metadata.
// Existing migration statements and their checksums remain unchanged.
func pathAssetStatements() []string {
	create := strings.Replace(assetCatalogStatements[0], "CREATE TABLE assets (", "CREATE TABLE assets_next (", 1)
	create = strings.Replace(create, "'template', 'clip', 'macro'", "'template', 'clip', 'macro', 'path'", 1)
	return []string{
		`CREATE TABLE path_migration_variants AS SELECT * FROM asset_variants`,
		`CREATE TABLE path_migration_tags AS SELECT * FROM asset_tags`,
		`DROP TABLE asset_variants`,
		`DROP TABLE asset_tags`,
		create,
		`INSERT INTO assets_next SELECT * FROM assets`,
		`DROP TABLE assets`,
		`ALTER TABLE assets_next RENAME TO assets`,
		assetCatalogStatements[1],
		assetCatalogStatements[2],
		`INSERT INTO asset_variants SELECT * FROM path_migration_variants`,
		`INSERT INTO asset_tags SELECT * FROM path_migration_tags`,
		`DROP TABLE path_migration_variants`,
		`DROP TABLE path_migration_tags`,
		`CREATE INDEX idx_assets_kind_name ON assets(kind, name COLLATE NOCASE, guid)`,
		`CREATE INDEX idx_assets_kind_created ON assets(kind, created_at DESC, guid)`,
		`CREATE INDEX idx_asset_tags_normalized ON asset_tags(normalized_tag, asset_guid)`,
	}
}
