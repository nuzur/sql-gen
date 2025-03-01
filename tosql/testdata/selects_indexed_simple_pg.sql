

-- user selects:
-- name: FetchUserByUuidAndVersion :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "uuid" = ? AND "version" = ? ;

        
-- name: FetchUserByEmail :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? 
LIMIT ?, ?;
        
-- name: FetchUserByStatus :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "status" = ? 
LIMIT ?, ?;
        
-- name: FetchUserByUuidAndVersionForUpdate :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "uuid" = ? AND "version" = ? 
FOR UPDATE;
        
-- name: FetchUserByEmailOrderedByUpdatedAtASC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ?  
ORDER BY updated_at ASC
LIMIT ?, ?;

-- name: FetchUserByEmailOrderedByUpdatedAtDESC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ?  
ORDER BY updated_at DESC
LIMIT ?, ?;

            
-- name: FetchUserByStatusOrderedByUpdatedAtASC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "status" = ?  
ORDER BY updated_at ASC
LIMIT ?, ?;

-- name: FetchUserByStatusOrderedByUpdatedAtDESC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "status" = ?  
ORDER BY updated_at DESC
LIMIT ?, ?;

            




-- folder selects:
-- name: FetchFolderByUuid :many
SELECT "uuid","version","status","created_at","updated_at","created_by","updated_by"
FROM "folder"
WHERE 
    "uuid" = ? ;

        
-- name: FetchFolderByUuidForUpdate :many
SELECT "uuid","version","status","created_at","updated_at","created_by","updated_by"
FROM "folder"
WHERE 
    "uuid" = ? 
FOR UPDATE;
        




-- post selects:
-- name: FetchPostByUuid :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "uuid" = ? ;

        
-- name: FetchPostBySlug :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "slug" = ? 
LIMIT ?, ?;
        
-- name: FetchPostByUuidForUpdate :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "uuid" = ? 
FOR UPDATE;
        


