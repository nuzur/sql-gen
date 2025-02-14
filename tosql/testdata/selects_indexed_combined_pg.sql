

-- user selects: 
-- name: FetchUserByUuid :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "uuid" = ? ;
        
     
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
     
-- name: FetchUserByEmailAndStatus :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "status" = ? 
LIMIT ?, ?;        
    
-- name: FetchUserByUuidForUpdate :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "uuid" = ? 
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

            
-- name: FetchUserByEmailAndStatusOrderedByUpdatedAtASC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "status" = ?  
ORDER BY updated_at ASC
LIMIT ?, ?;

-- name: FetchUserByEmailAndStatusOrderedByUpdatedAtDESC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "status" = ?  
ORDER BY updated_at DESC
LIMIT ?, ?;

            




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
        


