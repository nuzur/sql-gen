-- name: UpdateUser :exec
UPDATE "user"
SET
"version" = ?, "email" = ?, "password" = ?, "status" = ?, "created_at" = ?, "updated_at" = ?, "created_by" = ?, "updated_by" = ?
WHERE
"uuid" = ?;

-- name: UpdatePost :exec
UPDATE "post"
SET
"version" = ?, "title" = ?, "slug" = ?, "description" = ?, "content" = ?, "status" = ?, "created_at" = ?, "updated_at" = ?, "created_by" = ?, "updated_by" = ?, "media" = ?, "user_uuid" = ?
WHERE
"uuid" = ?;

