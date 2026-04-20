-- +goose Up
-- +goose StatementBegin

-- Convert prize_details from unstructured TEXT to structured JSONB array.
-- Each element is: {"rank": <int>, "label": "<string>", "description": "<string>"}
-- Existing free-text entries are cleared (they were not valid JSON).
ALTER TABLE games
    ALTER COLUMN prize_details TYPE JSONB
    USING CASE
        WHEN prize_details IS NULL THEN NULL
        ELSE '[]'::jsonb
    END;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE games
    ALTER COLUMN prize_details TYPE TEXT
    USING prize_details::text;

-- +goose StatementEnd
