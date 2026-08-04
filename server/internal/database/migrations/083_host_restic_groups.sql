-- Cached list of resticprofile.yaml "groups" section group names reported
-- by the agent (mirrors restic_profiles from migration 081) so the
-- backup/scheduled-task pickers can also offer a group — resticprofile
-- resolves a group the same way as a profile when passed to `--name`.
ALTER TABLE hosts ADD COLUMN restic_groups jsonb DEFAULT '[]'::jsonb NOT NULL;
