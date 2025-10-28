-- PGTransfer Sample Data Script
-- This script populates the test database with realistic sample data

-- Set search path
SET search_path TO public, sales, inventory, hr;

-- ============================================================================
-- PUBLIC SCHEMA DATA
-- ============================================================================

-- Insert sample users
INSERT INTO public.users (username, email, first_name, last_name, date_of_birth, is_active) VALUES
('john_doe', 'john.doe@example.com', 'John', 'Doe', '1985-03-15', true),
('jane_smith', 'jane.smith@example.com', 'Jane', 'Smith', '1990-07-22', true),
('bob_wilson', 'bob.wilson@example.com', 'Bob', 'Wilson', '1988-11-08', true),
('alice_brown', 'alice.brown@example.com', 'Alice', 'Brown', '1992-01-30', true),
('charlie_davis', 'charlie.davis@example.com', 'Charlie', 'Davis', '1987-09-12', false),
('diana_miller', 'diana.miller@example.com', 'Diana', 'Miller', '1991-05-18', true),
('frank_garcia', 'frank.garcia@example.com', 'Frank', 'Garcia', '1989-12-03', true),
('grace_lee', 'grace.lee@example.com', 'Grace', 'Lee', '1993-04-25', true),
('henry_taylor', 'henry.taylor@example.com', 'Henry', 'Taylor', '1986-08-14', true),
('ivy_anderson', 'ivy.anderson@example.com', 'Ivy', 'Anderson', '1994-02-07', true);

-- Insert sample posts
INSERT INTO public.posts (user_id, title, content, status, view_count) VALUES
(1, 'Getting Started with PostgreSQL', 'PostgreSQL is a powerful, open source object-relational database system...', 'published', 1250),
(2, 'Advanced SQL Techniques', 'In this post, we will explore advanced SQL techniques including CTEs, window functions...', 'published', 890),
(3, 'Database Performance Optimization', 'Performance optimization is crucial for any database application...', 'published', 2100),
(1, 'Understanding Database Indexes', 'Indexes are essential for database performance...', 'published', 750),
(4, 'Data Migration Best Practices', 'When migrating data between systems, there are several best practices to follow...', 'draft', 0),
(2, 'SQL vs NoSQL: When to Use What', 'The choice between SQL and NoSQL databases depends on various factors...', 'published', 1680),
(5, 'Database Security Fundamentals', 'Security should be a top priority when designing database systems...', 'archived', 320),
(3, 'Backup and Recovery Strategies', 'Having a solid backup and recovery strategy is essential...', 'published', 945),
(6, 'Working with JSON in PostgreSQL', 'PostgreSQL provides excellent support for JSON data types...', 'published', 1120),
(7, 'Database Design Patterns', 'Good database design is the foundation of any successful application...', 'published', 1450);

-- Insert sample comments
INSERT INTO public.comments (post_id, user_id, content, is_approved) VALUES
(1, 2, 'Great introduction! Very helpful for beginners.', true),
(1, 3, 'Thanks for sharing this. The examples are clear and easy to follow.', true),
(2, 1, 'Excellent coverage of advanced topics. Looking forward to more posts like this.', true),
(2, 4, 'The window functions section was particularly useful.', true),
(3, 5, 'Performance optimization is indeed crucial. Have you considered covering query planning?', true),
(3, 6, 'This helped me optimize my slow queries. Thank you!', true),
(4, 7, 'Indexes can be tricky. This explanation makes it much clearer.', true),
(6, 8, 'The comparison table is very helpful for decision making.', true),
(8, 9, 'Backup strategies saved my project last month. Great advice!', true),
(9, 10, 'JSON support in PostgreSQL is amazing. Thanks for the examples.', true);

-- ============================================================================
-- SALES SCHEMA DATA
-- ============================================================================

-- Insert sample customers
INSERT INTO sales.customers (customer_code, company_name, contact_name, contact_email, phone, address, city, country, postal_code, credit_limit) VALUES
('CUST001', 'Tech Solutions Inc.', 'Michael Johnson', 'michael@techsolutions.com', '+1-555-0101', '123 Tech Street', 'San Francisco', 'USA', '94105', 50000.00),
('CUST002', 'Global Enterprises Ltd.', 'Sarah Williams', 'sarah@globalent.com', '+1-555-0102', '456 Business Ave', 'New York', 'USA', '10001', 75000.00),
('CUST003', 'Innovation Corp', 'David Chen', 'david@innovation.com', '+1-555-0103', '789 Innovation Blvd', 'Austin', 'USA', '73301', 30000.00),
('CUST004', 'Future Systems', 'Lisa Rodriguez', 'lisa@futuresys.com', '+1-555-0104', '321 Future Lane', 'Seattle', 'USA', '98101', 60000.00),
('CUST005', 'Digital Dynamics', 'Robert Kim', 'robert@digitaldyn.com', '+1-555-0105', '654 Digital Drive', 'Los Angeles', 'USA', '90210', 40000.00),
('CUST006', 'Smart Solutions', 'Emma Thompson', 'emma@smartsol.com', '+44-20-7946-0958', '10 Smart Street', 'London', 'UK', 'SW1A 1AA', 35000.00),
('CUST007', 'Quantum Technologies', 'James Wilson', 'james@quantum.com', '+1-555-0107', '987 Quantum Way', 'Boston', 'USA', '02101', 80000.00),
('CUST008', 'NextGen Industries', 'Maria Garcia', 'maria@nextgen.com', '+1-555-0108', '147 NextGen Plaza', 'Chicago', 'USA', '60601', 45000.00);

-- Insert sample orders
INSERT INTO sales.orders (order_number, customer_id, order_date, required_date, shipped_date, status, total_amount, notes) VALUES
('ORD-2024-001', 1, '2024-01-15', '2024-01-25', '2024-01-20', 'delivered', 15750.00, 'Rush order for Q1 project'),
('ORD-2024-002', 2, '2024-01-18', '2024-02-01', '2024-01-28', 'delivered', 28900.00, 'Standard delivery'),
('ORD-2024-003', 3, '2024-01-22', '2024-02-05', NULL, 'processing', 12300.00, 'Waiting for inventory'),
('ORD-2024-004', 1, '2024-01-25', '2024-02-08', NULL, 'pending', 8750.00, 'Follow-up order'),
('ORD-2024-005', 4, '2024-01-28', '2024-02-12', NULL, 'processing', 22100.00, 'Large enterprise order'),
('ORD-2024-006', 5, '2024-02-01', '2024-02-15', NULL, 'pending', 16800.00, 'New customer order'),
('ORD-2024-007', 2, '2024-02-03', '2024-02-18', NULL, 'processing', 31200.00, 'Quarterly replenishment'),
('ORD-2024-008', 6, '2024-02-05', '2024-02-20', NULL, 'pending', 19500.00, 'International shipping'),
('ORD-2024-009', 7, '2024-02-08', '2024-02-22', NULL, 'pending', 45600.00, 'Enterprise license renewal'),
('ORD-2024-010', 3, '2024-02-10', '2024-02-25', NULL, 'pending', 13900.00, 'Additional modules');

-- Insert sample order items
INSERT INTO sales.order_items (order_id, product_code, product_name, quantity, unit_price, discount_percent) VALUES
-- Order 1 items
(1, 'SOFT-001', 'Database Management Software', 5, 2500.00, 10.00),
(1, 'LIC-001', 'Annual Support License', 5, 750.00, 5.00),
-- Order 2 items
(2, 'SOFT-002', 'Analytics Platform', 3, 8500.00, 15.00),
(2, 'TRAIN-001', 'Training Package', 1, 3500.00, 0.00),
-- Order 3 items
(3, 'SOFT-003', 'Reporting Tools', 10, 1200.00, 8.00),
(3, 'SUP-001', 'Premium Support', 1, 300.00, 0.00),
-- Order 4 items
(4, 'SOFT-001', 'Database Management Software', 2, 2500.00, 15.00),
(4, 'ADD-001', 'Additional Modules', 3, 1250.00, 5.00),
-- Order 5 items
(5, 'ENT-001', 'Enterprise Suite', 1, 20000.00, 12.00),
(5, 'IMPL-001', 'Implementation Service', 1, 2500.00, 0.00);

-- ============================================================================
-- INVENTORY SCHEMA DATA
-- ============================================================================

-- Insert sample products
INSERT INTO inventory.products (product_code, product_name, description, category, unit_price, cost_price, stock_quantity, reorder_level, is_active) VALUES
('SOFT-001', 'Database Management Software', 'Professional database management and administration software', 'Software', 2500.00, 1200.00, 50, 10, true),
('SOFT-002', 'Analytics Platform', 'Advanced data analytics and visualization platform', 'Software', 8500.00, 4200.00, 25, 5, true),
('SOFT-003', 'Reporting Tools', 'Comprehensive reporting and dashboard creation tools', 'Software', 1200.00, 600.00, 100, 20, true),
('LIC-001', 'Annual Support License', 'Annual technical support and maintenance license', 'License', 750.00, 300.00, 200, 50, true),
('TRAIN-001', 'Training Package', 'Comprehensive training program for software products', 'Service', 3500.00, 1500.00, 15, 3, true),
('SUP-001', 'Premium Support', 'Premium 24/7 technical support service', 'Service', 300.00, 120.00, 500, 100, true),
('ENT-001', 'Enterprise Suite', 'Complete enterprise software solution', 'Software', 20000.00, 10000.00, 10, 2, true),
('ADD-001', 'Additional Modules', 'Add-on modules for existing software', 'Software', 1250.00, 625.00, 75, 15, true),
('IMPL-001', 'Implementation Service', 'Professional implementation and setup service', 'Service', 2500.00, 1000.00, 30, 5, true),
('CONS-001', 'Consulting Hours', 'Professional consulting services per hour', 'Service', 150.00, 75.00, 1000, 200, true);

-- Insert sample stock movements
INSERT INTO inventory.stock_movements (product_id, movement_type, quantity, reference_number, notes, movement_date) VALUES
(1, 'in', 100, 'PO-2024-001', 'Initial stock purchase', '2024-01-01 09:00:00'),
(2, 'in', 50, 'PO-2024-002', 'Initial stock purchase', '2024-01-01 09:15:00'),
(3, 'in', 150, 'PO-2024-003', 'Initial stock purchase', '2024-01-01 09:30:00'),
(1, 'out', 25, 'ORD-2024-001', 'Sales order fulfillment', '2024-01-15 14:30:00'),
(2, 'out', 15, 'ORD-2024-002', 'Sales order fulfillment', '2024-01-18 11:45:00'),
(3, 'out', 30, 'ORD-2024-003', 'Sales order fulfillment', '2024-01-22 16:20:00'),
(1, 'out', 10, 'ORD-2024-004', 'Sales order fulfillment', '2024-01-25 10:15:00'),
(7, 'in', 15, 'PO-2024-004', 'Restocking enterprise products', '2024-02-01 08:00:00'),
(8, 'adjustment', -5, 'ADJ-2024-001', 'Inventory count adjustment', '2024-02-05 17:00:00'),
(9, 'out', 10, 'ORD-2024-005', 'Service delivery', '2024-02-08 13:30:00');

-- ============================================================================
-- HR SCHEMA DATA
-- ============================================================================

-- Insert sample employees
INSERT INTO hr.employees (employee_id, first_name, last_name, email, phone, department, position, salary, hire_date, birth_date, address, is_active, manager_id) VALUES
('EMP001', 'John', 'Manager', 'john.manager@company.com', '+1-555-1001', 'Management', 'CEO', 150000.00, '2020-01-15', '1975-03-20', '100 Executive Drive', true, NULL),
('EMP002', 'Sarah', 'Director', 'sarah.director@company.com', '+1-555-1002', 'Sales', 'Sales Director', 120000.00, '2020-03-01', '1980-07-15', '200 Sales Street', true, 1),
('EMP003', 'Mike', 'Developer', 'mike.dev@company.com', '+1-555-1003', 'Engineering', 'Senior Developer', 95000.00, '2021-06-15', '1985-11-22', '300 Code Avenue', true, 1),
('EMP004', 'Lisa', 'Analyst', 'lisa.analyst@company.com', '+1-555-1004', 'Analytics', 'Data Analyst', 75000.00, '2021-09-01', '1990-02-10', '400 Data Lane', true, 1),
('EMP005', 'David', 'Representative', 'david.sales@company.com', '+1-555-1005', 'Sales', 'Sales Representative', 65000.00, '2022-01-10', '1988-05-30', '500 Customer Road', true, 2),
('EMP006', 'Emma', 'Developer', 'emma.dev@company.com', '+1-555-1006', 'Engineering', 'Junior Developer', 70000.00, '2022-04-01', '1992-09-18', '600 Programming Place', true, 3),
('EMP007', 'James', 'Specialist', 'james.support@company.com', '+1-555-1007', 'Support', 'Technical Support Specialist', 55000.00, '2022-07-15', '1987-12-05', '700 Help Desk Highway', true, 1),
('EMP008', 'Maria', 'Coordinator', 'maria.hr@company.com', '+1-555-1008', 'HR', 'HR Coordinator', 60000.00, '2023-02-01', '1991-04-12', '800 Human Resources Row', true, 1),
('EMP009', 'Robert', 'Manager', 'robert.ops@company.com', '+1-555-1009', 'Operations', 'Operations Manager', 85000.00, '2023-05-15', '1983-08-25', '900 Operations Oval', true, 1),
('EMP010', 'Jennifer', 'Intern', 'jennifer.intern@company.com', '+1-555-1010', 'Engineering', 'Software Engineering Intern', 35000.00, '2023-09-01', '1998-01-14', '1000 Intern Circle', true, 3);

-- Insert sample attendance data (last 30 days)
INSERT INTO hr.attendance (employee_id, attendance_date, check_in_time, check_out_time, hours_worked, status) VALUES
-- Employee 1 (John Manager)
(1, CURRENT_DATE - INTERVAL '29 days', '08:00:00', '17:30:00', 8.5, 'present'),
(1, CURRENT_DATE - INTERVAL '28 days', '08:15:00', '17:45:00', 8.5, 'present'),
(1, CURRENT_DATE - INTERVAL '27 days', '08:00:00', '18:00:00', 9.0, 'present'),
-- Employee 2 (Sarah Director)
(2, CURRENT_DATE - INTERVAL '29 days', '08:30:00', '17:00:00', 8.0, 'present'),
(2, CURRENT_DATE - INTERVAL '28 days', '09:00:00', '17:30:00', 7.5, 'late'),
(2, CURRENT_DATE - INTERVAL '27 days', '08:15:00', '17:15:00', 8.0, 'present'),
-- Employee 3 (Mike Developer)
(3, CURRENT_DATE - INTERVAL '29 days', '09:00:00', '18:00:00', 8.0, 'present'),
(3, CURRENT_DATE - INTERVAL '28 days', '09:15:00', '18:15:00', 8.0, 'present'),
(3, CURRENT_DATE - INTERVAL '27 days', NULL, NULL, 0.0, 'absent'),
-- Employee 4 (Lisa Analyst)
(4, CURRENT_DATE - INTERVAL '29 days', '08:45:00', '17:45:00', 8.0, 'present'),
(4, CURRENT_DATE - INTERVAL '28 days', '08:30:00', '12:30:00', 4.0, 'half_day'),
(4, CURRENT_DATE - INTERVAL '27 days', '08:00:00', '17:00:00', 8.0, 'present'),
-- Employee 5 (David Sales Rep)
(5, CURRENT_DATE - INTERVAL '29 days', '08:00:00', '17:00:00', 8.0, 'present'),
(5, CURRENT_DATE - INTERVAL '28 days', '08:00:00', '17:00:00', 8.0, 'present'),
(5, CURRENT_DATE - INTERVAL '27 days', '08:30:00', '17:30:00', 8.0, 'present');

-- ============================================================================
-- SUMMARY STATISTICS
-- ============================================================================

-- Display summary of inserted data
DO $$
BEGIN
    RAISE NOTICE 'Sample data insertion completed successfully!';
    RAISE NOTICE 'Summary:';
    RAISE NOTICE '- Users: % records', (SELECT COUNT(*) FROM public.users);
    RAISE NOTICE '- Posts: % records', (SELECT COUNT(*) FROM public.posts);
    RAISE NOTICE '- Comments: % records', (SELECT COUNT(*) FROM public.comments);
    RAISE NOTICE '- Customers: % records', (SELECT COUNT(*) FROM sales.customers);
    RAISE NOTICE '- Orders: % records', (SELECT COUNT(*) FROM sales.orders);
    RAISE NOTICE '- Order Items: % records', (SELECT COUNT(*) FROM sales.order_items);
    RAISE NOTICE '- Products: % records', (SELECT COUNT(*) FROM inventory.products);
    RAISE NOTICE '- Stock Movements: % records', (SELECT COUNT(*) FROM inventory.stock_movements);
    RAISE NOTICE '- Employees: % records', (SELECT COUNT(*) FROM hr.employees);
    RAISE NOTICE '- Attendance Records: % records', (SELECT COUNT(*) FROM hr.attendance);
END $$;