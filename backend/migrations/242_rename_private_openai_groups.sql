-- Rename legacy private OpenAI group names to the platform-specific format.
-- Fail rather than merging a legacy group into an existing target name.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM groups AS legacy
        JOIN groups AS current
          ON current.name = 'Private-openai-USR' || substring(legacy.name FROM 12)
         AND current.deleted_at IS NULL
        WHERE legacy.name ~ '^private-usr[0-9]+$'
          AND legacy.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'Cannot rename private OpenAI group: target name already exists';
    END IF;

    UPDATE groups
    SET name = 'Private-openai-USR' || substring(name FROM 12)
    WHERE name ~ '^private-usr[0-9]+$'
      AND deleted_at IS NULL;
END $$;
