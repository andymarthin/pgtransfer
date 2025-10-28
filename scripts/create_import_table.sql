-- Create table for importing 1M records
-- This table handles the exported CSV format with timestamps and text fields

DROP TABLE IF EXISTS public.test_1m_users_imported;

CREATE TABLE public.test_1m_users_imported (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    email VARCHAR(100) NOT NULL,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    date_of_birth TEXT, -- Using TEXT to handle timestamp format
    is_active BOOLEAN DEFAULT true,
    created_at TEXT, -- Using TEXT to handle timestamp format
    department VARCHAR(50),
    salary TEXT, -- Using TEXT to handle byte array format
    phone VARCHAR(20),
    country_code TEXT, -- Using TEXT to handle byte array format
    city VARCHAR(50),
    postal_code VARCHAR(10)
);

-- Create indexes for better performance
CREATE INDEX CONCURRENTLY idx_imported_users_username ON public.test_1m_users_imported(username);
CREATE INDEX CONCURRENTLY idx_imported_users_email ON public.test_1m_users_imported(email);
CREATE INDEX CONCURRENTLY idx_imported_users_department ON public.test_1m_users_imported(department);
CREATE INDEX CONCURRENTLY idx_imported_users_is_active ON public.test_1m_users_imported(is_active);

-- Display table info
\d public.test_1m_users_imported;

SELECT 'Import table created successfully with flexible text fields!' as status;