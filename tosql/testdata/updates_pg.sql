-- name: UpdateUser :exec
UPDATE "user"
SET
"uuid" = ?, "version" = ?, "email" = ?, "password" = ?, "status" = ?, "created_at" = ?, "updated_at" = ?, "created_by" = ?, "updated_by" = ?
WHERE
"uuid" = ? AND "version" = ?;

-- name: UpdatePost :exec
UPDATE "post"
SET
"uuid" = ?, "version" = ?, "title" = ?, "slug" = ?, "description" = ?, "content" = ?, "status" = ?, "created_at" = ?, "updated_at" = ?, "created_by" = ?, "updated_by" = ?, "media" = ?, "user_uuid" = ?
WHERE
"uuid" = ?;

-- name: UpdateFolder :exec
UPDATE "folder"
SET
"uuid" = ?, "version" = ?, "status" = ?, "created_at" = ?, "updated_at" = ?, "created_by" = ?, "updated_by" = ?
WHERE
"uuid" = ?;

