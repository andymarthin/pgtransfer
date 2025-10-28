-- Generate 50,000 test records for batch processing testing
-- This script creates a test_large_users table and populates it with synthetic data

-- Drop table if exists
DROP TABLE IF EXISTS public.test_large_users;

-- Create test table
CREATE TABLE public.test_large_users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    email VARCHAR(100) NOT NULL,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    date_of_birth DATE,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    department VARCHAR(50),
    salary DECIMAL(10,2),
    phone VARCHAR(20)
);

-- Generate 50,000 test records using generate_series
INSERT INTO public.test_large_users (
    username, 
    email, 
    first_name, 
    last_name, 
    date_of_birth, 
    is_active, 
    department, 
    salary, 
    phone
)
SELECT 
    'user_' || LPAD(i::text, 6, '0') as username,
    'user' || i || '@example.com' as email,
    CASE (i % 20)
        WHEN 0 THEN 'John'
        WHEN 1 THEN 'Jane'
        WHEN 2 THEN 'Michael'
        WHEN 3 THEN 'Sarah'
        WHEN 4 THEN 'David'
        WHEN 5 THEN 'Emily'
        WHEN 6 THEN 'Robert'
        WHEN 7 THEN 'Jessica'
        WHEN 8 THEN 'William'
        WHEN 9 THEN 'Ashley'
        WHEN 10 THEN 'James'
        WHEN 11 THEN 'Amanda'
        WHEN 12 THEN 'Christopher'
        WHEN 13 THEN 'Stephanie'
        WHEN 14 THEN 'Daniel'
        WHEN 15 THEN 'Jennifer'
        WHEN 16 THEN 'Matthew'
        WHEN 17 THEN 'Nicole'
        WHEN 18 THEN 'Anthony'
        ELSE 'Michelle'
    END as first_name,
    CASE (i % 15)
        WHEN 0 THEN 'Smith'
        WHEN 1 THEN 'Johnson'
        WHEN 2 THEN 'Williams'
        WHEN 3 THEN 'Brown'
        WHEN 4 THEN 'Jones'
        WHEN 5 THEN 'Garcia'
        WHEN 6 THEN 'Miller'
        WHEN 7 THEN 'Davis'
        WHEN 8 THEN 'Rodriguez'
        WHEN 9 THEN 'Martinez'
        WHEN 10 THEN 'Hernandez'
        WHEN 11 THEN 'Lopez'
        WHEN 12 THEN 'Gonzalez'
        WHEN 13 THEN 'Wilson'
        ELSE 'Anderson'
    END as last_name,
    DATE '1970-01-01' + (i % 18250) as date_of_birth, -- Random dates over ~50 years
    (i % 10) != 0 as is_active, -- 90% active users
    CASE (i % 8)
        WHEN 0 THEN 'Engineering'
        WHEN 1 THEN 'Marketing'
        WHEN 2 THEN 'Sales'
        WHEN 3 THEN 'HR'
        WHEN 4 THEN 'Finance'
        WHEN 5 THEN 'Operations'
        WHEN 6 THEN 'Support'
        ELSE 'Management'
    END as department,
    30000 + (i % 100000) + (RANDOM() * 20000)::int as salary, -- Salary between 30k-150k
    '+1-555-' || LPAD(((i % 9000) + 1000)::text, 4, '0') as phone
FROM generate_series(1, 50000) as i;

-- Create indexes for better performance
CREATE INDEX idx_test_large_users_username ON public.test_large_users(username);
CREATE INDEX idx_test_large_users_email ON public.test_large_users(email);
CREATE INDEX idx_test_large_users_department ON public.test_large_users(department);
CREATE INDEX idx_test_large_users_is_active ON public.test_large_users(is_active);

-- Display summary
SELECT 
    COUNT(*) as total_records,
    COUNT(CASE WHEN is_active THEN 1 END) as active_users,
    COUNT(DISTINCT department) as departments,
    MIN(salary) as min_salary,
    MAX(salary) as max_salary,
    AVG(salary)::int as avg_salary
FROM public.test_large_users;

-- Show sample data
SELECT * FROM public.test_large_users LIMIT 10;