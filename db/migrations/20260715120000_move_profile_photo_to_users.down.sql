UPDATE files
SET purpose = 'provider_profile_photo'
WHERE purpose = 'profile_photo';

ALTER TABLE providers
    ADD COLUMN profile_photo_file_id UUID REFERENCES files(id);

UPDATE providers p
SET profile_photo_file_id = u.profile_photo_file_id
FROM users u
WHERE u.id = p.user_id;

CREATE INDEX providers_profile_photo_file_id_idx
    ON providers (profile_photo_file_id);

DROP INDEX users_profile_photo_file_id_idx;

ALTER TABLE users
    DROP COLUMN profile_photo_file_id;
