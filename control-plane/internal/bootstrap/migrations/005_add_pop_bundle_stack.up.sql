-- Add pop-bundle to deployment_stack enum
-- Migration: 005_add_pop_bundle_stack.up.sql

-- Add 'pop-bundle' value to the deployment_stack enum type
ALTER TYPE deployment_stack ADD VALUE IF NOT EXISTS 'pop-bundle';
