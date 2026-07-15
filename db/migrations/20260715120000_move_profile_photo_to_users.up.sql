ALTER TABLE users
    ADD COLUMN profile_photo_file_id UUID REFERENCES files(id);

UPDATE users u
SET profile_photo_file_id = p.profile_photo_file_id
FROM providers p
WHERE p.user_id = u.id;

CREATE INDEX users_profile_photo_file_id_idx
    ON users (profile_photo_file_id);

DROP INDEX providers_profile_photo_file_id_idx;

ALTER TABLE providers
    DROP COLUMN profile_photo_file_id;

UPDATE files
SET purpose = 'profile_photo'
WHERE purpose = 'provider_profile_photo';
