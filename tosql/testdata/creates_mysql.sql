CREATE TABLE IF NOT EXISTS `user` (
    `uuid` CHAR(36) NOT NULL,
    `version` INT NOT NULL,
    `email` VARCHAR(512),
    `password` VARCHAR(255),
    `status` INT NOT NULL,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_by` CHAR(36) NOT NULL,
    `updated_by` CHAR(36) NOT NULL,
    PRIMARY KEY (`uuid`, `version`),
    INDEX `index_email` (`email`),
    INDEX `index_status` (`status`),
    INDEX `index_updated_at` (`updated_at`),
    UNIQUE INDEX `unique_uuid` (`uuid`),
    UNIQUE INDEX `unique_email` (`email`)
) ENGINE = InnoDB;

CREATE TABLE IF NOT EXISTS `folder` (
    `uuid` CHAR(36) NOT NULL,
    `version` INT NOT NULL,
    `status` INT NOT NULL,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_by` CHAR(36) NOT NULL,
    `updated_by` CHAR(36) NOT NULL
) ENGINE = InnoDB;

CREATE TABLE IF NOT EXISTS `single_key` (
    `uuid` CHAR(36) NOT NULL,
    `version` INT NOT NULL,
    `status` INT NOT NULL,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_by` CHAR(36) NOT NULL,
    `updated_by` CHAR(36) NOT NULL,
    PRIMARY KEY (`uuid`),
    INDEX `nuevo_indice` (`version`)
) ENGINE = InnoDB;

CREATE TABLE IF NOT EXISTS `post` (
    `uuid` CHAR(36) NOT NULL,
    `version` INT NOT NULL,
    `title` VARCHAR(255) NOT NULL,
    `slug` VARCHAR(512) NOT NULL,
    `description` VARCHAR(255),
    `content` TEXT,
    `status` INT NOT NULL,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_by` CHAR(36) NOT NULL,
    `updated_by` CHAR(36) NOT NULL,
    `media` JSON NOT NULL,
    `user_uuid` CHAR(36) NOT NULL,
    PRIMARY KEY (`uuid`),
    INDEX `nuevo_indice` (`slug`),
    UNIQUE INDEX `unique_slug` (`slug`),
    CONSTRAINT `post_user`
        FOREIGN KEY (`user_uuid`)
        REFERENCES `user` (`uuid`)
) ENGINE = InnoDB;

