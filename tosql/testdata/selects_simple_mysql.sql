-- name: FetchUser :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`;

-- name: FetchFolder :many
SELECT `uuid`,`version`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `folder`;

-- name: FetchPost :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`;

