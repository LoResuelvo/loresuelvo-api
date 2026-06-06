ALTER TABLE providers
    ADD COLUMN profile_photo_file_id UUID REFERENCES files(id);

CREATE INDEX providers_profile_photo_file_id_idx
    ON providers (profile_photo_file_id);
