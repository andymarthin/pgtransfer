-- Generate 1,000,000 test records for stress testing batch processing
-- This script creates a test_1m_users table and populates it with synthetic data

-- Drop table if exists
DROP TABLE IF EXISTS public.test_1m_users;

-- Create test table with optimized structure for large datasets
CREATE TABLE public.test_1m_users (
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
    phone VARCHAR(20),
    country_code CHAR(2),
    city VARCHAR(50),
    postal_code VARCHAR(10)
);

-- Generate 1,000,000 test records using generate_series
-- Using optimized approach for large dataset generation
INSERT INTO public.test_1m_users (
    username, 
    email, 
    first_name, 
    last_name, 
    date_of_birth, 
    is_active, 
    department, 
    salary, 
    phone,
    country_code,
    city,
    postal_code
)
SELECT 
    'user_' || LPAD(i::text, 7, '0') as username,
    'user' || i || '@company' || ((i % 100) + 1) || '.com' as email,
    CASE (i % 25)
        WHEN 0 THEN 'John' WHEN 1 THEN 'Jane' WHEN 2 THEN 'Michael' WHEN 3 THEN 'Sarah' WHEN 4 THEN 'David'
        WHEN 5 THEN 'Emily' WHEN 6 THEN 'Robert' WHEN 7 THEN 'Jessica' WHEN 8 THEN 'William' WHEN 9 THEN 'Ashley'
        WHEN 10 THEN 'James' WHEN 11 THEN 'Amanda' WHEN 12 THEN 'Christopher' WHEN 13 THEN 'Stephanie' WHEN 14 THEN 'Daniel'
        WHEN 15 THEN 'Jennifer' WHEN 16 THEN 'Matthew' WHEN 17 THEN 'Nicole' WHEN 18 THEN 'Anthony' WHEN 19 THEN 'Michelle'
        WHEN 20 THEN 'Mark' WHEN 21 THEN 'Lisa' WHEN 22 THEN 'Steven' WHEN 23 THEN 'Karen' ELSE 'Brian'
    END as first_name,
    CASE (i % 20)
        WHEN 0 THEN 'Smith' WHEN 1 THEN 'Johnson' WHEN 2 THEN 'Williams' WHEN 3 THEN 'Brown' WHEN 4 THEN 'Jones'
        WHEN 5 THEN 'Garcia' WHEN 6 THEN 'Miller' WHEN 7 THEN 'Davis' WHEN 8 THEN 'Rodriguez' WHEN 9 THEN 'Martinez'
        WHEN 10 THEN 'Hernandez' WHEN 11 THEN 'Lopez' WHEN 12 THEN 'Gonzalez' WHEN 13 THEN 'Wilson' WHEN 14 THEN 'Anderson'
        WHEN 15 THEN 'Thomas' WHEN 16 THEN 'Taylor' WHEN 17 THEN 'Moore' WHEN 18 THEN 'Jackson' ELSE 'Martin'
    END as last_name,
    DATE '1960-01-01' + (i % 22000) as date_of_birth, -- Random dates over ~60 years
    (i % 10) != 0 as is_active, -- 90% active users
    CASE (i % 12)
        WHEN 0 THEN 'Engineering' WHEN 1 THEN 'Marketing' WHEN 2 THEN 'Sales' WHEN 3 THEN 'HR'
        WHEN 4 THEN 'Finance' WHEN 5 THEN 'Operations' WHEN 6 THEN 'Support' WHEN 7 THEN 'Management'
        WHEN 8 THEN 'Research' WHEN 9 THEN 'Legal' WHEN 10 THEN 'IT' ELSE 'Consulting'
    END as department,
    25000 + (i % 150000) + ((i * 17) % 50000) as salary, -- Salary between 25k-225k with variation
    '+' || ((i % 50) + 1) || '-555-' || LPAD(((i % 9000) + 1000)::text, 4, '0') as phone,
    CASE (i % 10)
        WHEN 0 THEN 'US' WHEN 1 THEN 'CA' WHEN 2 THEN 'UK' WHEN 3 THEN 'DE' WHEN 4 THEN 'FR'
        WHEN 5 THEN 'AU' WHEN 6 THEN 'JP' WHEN 7 THEN 'BR' WHEN 8 THEN 'IN' ELSE 'MX'
    END as country_code,
    CASE (i % 15)
        WHEN 0 THEN 'New York' WHEN 1 THEN 'Los Angeles' WHEN 2 THEN 'Chicago' WHEN 3 THEN 'Houston' WHEN 4 THEN 'Phoenix'
        WHEN 5 THEN 'Philadelphia' WHEN 6 THEN 'San Antonio' WHEN 7 THEN 'San Diego' WHEN 8 THEN 'Dallas' WHEN 9 THEN 'San Jose'
        WHEN 10 THEN 'Austin' WHEN 11 THEN 'Jacksonville' WHEN 12 THEN 'Fort Worth' WHEN 13 THEN 'Columbus' ELSE 'Charlotte'
    END as city,
    LPAD(((i % 99999) + 10000)::text, 5, '0') as postal_code
FROM generate_series(1, 1000000) as i;

-- Create indexes for better performance (but not during initial load)
CREATE INDEX CONCURRENTLY idx_test_1m_users_username ON public.test_1m_users(username);
CREATE INDEX CONCURRENTLY idx_test_1m_users_email ON public.test_1m_users(email);
CREATE INDEX CONCURRENTLY idx_test_1m_users_department ON public.test_1m_users(department);
CREATE INDEX CONCURRENTLY idx_test_1m_users_is_active ON public.test_1m_users(is_active);
CREATE INDEX CONCURRENTLY idx_test_1m_users_country ON public.test_1m_users(country_code);

-- Analyze table for better query planning
ANALYZE public.test_1m_users;

-- Display summary statistics
SELECT 
    COUNT(*) as total_records,
    COUNT(CASE WHEN is_active THEN 1 END) as active_users,
    COUNT(DISTINCT department) as departments,
    COUNT(DISTINCT country_code) as countries,
    MIN(salary) as min_salary,
    MAX(salary) as max_salary,
    AVG(salary)::int as avg_salary,
    pg_size_pretty(pg_total_relation_size('public.test_1m_users')) as table_size
FROM public.test_1m_users;

-- Show sample data
SELECT * FROM public.test_1m_users WHERE id <= 5 ORDER BY id;