DROP TABLE IF EXISTS skus CASCADE;
DROP TABLE IF EXISTS product_translations CASCADE;
DROP TABLE IF EXISTS products CASCADE;

-- Remove the product.publish permission + its role_permissions.
DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE key = 'product.publish');
DELETE FROM permissions WHERE key = 'product.publish';
