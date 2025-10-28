-- PGTransfer Test Database Initialization Script
-- This script creates the initial database schema and tables for testing

-- Create additional schemas for testing
CREATE SCHEMA IF NOT EXISTS sales;
CREATE SCHEMA IF NOT EXISTS inventory;
CREATE SCHEMA IF NOT EXISTS hr;

-- Set search path to include all schemas
SET search_path TO public, sales, inventory, hr;

-- ============================================================================
-- PUBLIC SCHEMA TABLES
-- ============================================================================

-- Users table
CREATE TABLE IF NOT EXISTS public.users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    date_of_birth DATE,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Posts table
CREATE TABLE IF NOT EXISTS public.posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES public.users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    content TEXT,
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    view_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Comments table
CREATE TABLE IF NOT EXISTS public.comments (
    id SERIAL PRIMARY KEY,
    post_id INTEGER REFERENCES public.posts(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES public.users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    is_approved BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- SALES SCHEMA TABLES
-- ============================================================================

-- Customers table
CREATE TABLE IF NOT EXISTS sales.customers (
    id SERIAL PRIMARY KEY,
    customer_code VARCHAR(20) UNIQUE NOT NULL,
    company_name VARCHAR(100) NOT NULL,
    contact_name VARCHAR(100),
    contact_email VARCHAR(100),
    phone VARCHAR(20),
    address TEXT,
    city VARCHAR(50),
    country VARCHAR(50),
    postal_code VARCHAR(20),
    credit_limit DECIMAL(12,2) DEFAULT 0.00,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Orders table
CREATE TABLE IF NOT EXISTS sales.orders (
    id SERIAL PRIMARY KEY,
    order_number VARCHAR(50) UNIQUE NOT NULL,
    customer_id INTEGER REFERENCES sales.customers(id),
    order_date DATE NOT NULL,
    required_date DATE,
    shipped_date DATE,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'shipped', 'delivered', 'cancelled')),
    total_amount DECIMAL(12,2) DEFAULT 0.00,
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Order items table
CREATE TABLE IF NOT EXISTS sales.order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES sales.orders(id) ON DELETE CASCADE,
    product_code VARCHAR(50) NOT NULL,
    product_name VARCHAR(100) NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price DECIMAL(10,2) NOT NULL CHECK (unit_price >= 0),
    discount_percent DECIMAL(5,2) DEFAULT 0.00 CHECK (discount_percent >= 0 AND discount_percent <= 100),
    line_total DECIMAL(12,2) GENERATED ALWAYS AS (quantity * unit_price * (1 - discount_percent / 100)) STORED
);

-- ============================================================================
-- INVENTORY SCHEMA TABLES
-- ============================================================================

-- Products table
CREATE TABLE IF NOT EXISTS inventory.products (
    id SERIAL PRIMARY KEY,
    product_code VARCHAR(50) UNIQUE NOT NULL,
    product_name VARCHAR(100) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    unit_price DECIMAL(10,2) NOT NULL CHECK (unit_price >= 0),
    cost_price DECIMAL(10,2) CHECK (cost_price >= 0),
    stock_quantity INTEGER DEFAULT 0 CHECK (stock_quantity >= 0),
    reorder_level INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Stock movements table
CREATE TABLE IF NOT EXISTS inventory.stock_movements (
    id SERIAL PRIMARY KEY,
    product_id INTEGER REFERENCES inventory.products(id),
    movement_type VARCHAR(20) NOT NULL CHECK (movement_type IN ('in', 'out', 'adjustment')),
    quantity INTEGER NOT NULL,
    reference_number VARCHAR(50),
    notes TEXT,
    movement_date TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(50) DEFAULT 'system'
);

-- ============================================================================
-- HR SCHEMA TABLES
-- ============================================================================

-- Employees table
CREATE TABLE IF NOT EXISTS hr.employees (
    id SERIAL PRIMARY KEY,
    employee_id VARCHAR(20) UNIQUE NOT NULL,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    phone VARCHAR(20),
    department VARCHAR(50),
    position VARCHAR(100),
    salary DECIMAL(10,2) CHECK (salary >= 0),
    hire_date DATE NOT NULL,
    birth_date DATE,
    address TEXT,
    is_active BOOLEAN DEFAULT true,
    manager_id INTEGER REFERENCES hr.employees(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Attendance table
CREATE TABLE IF NOT EXISTS hr.attendance (
    id SERIAL PRIMARY KEY,
    employee_id INTEGER REFERENCES hr.employees(id),
    attendance_date DATE NOT NULL,
    check_in_time TIME,
    check_out_time TIME,
    hours_worked DECIMAL(4,2),
    status VARCHAR(20) DEFAULT 'present' CHECK (status IN ('present', 'absent', 'late', 'half_day')),
    notes TEXT,
    UNIQUE(employee_id, attendance_date)
);

-- ============================================================================
-- INDEXES FOR PERFORMANCE
-- ============================================================================

-- Users indexes
CREATE INDEX IF NOT EXISTS idx_users_email ON public.users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON public.users(username);
CREATE INDEX IF NOT EXISTS idx_users_active ON public.users(is_active);

-- Posts indexes
CREATE INDEX IF NOT EXISTS idx_posts_user_id ON public.posts(user_id);
CREATE INDEX IF NOT EXISTS idx_posts_status ON public.posts(status);
CREATE INDEX IF NOT EXISTS idx_posts_created_at ON public.posts(created_at);

-- Comments indexes
CREATE INDEX IF NOT EXISTS idx_comments_post_id ON public.comments(post_id);
CREATE INDEX IF NOT EXISTS idx_comments_user_id ON public.comments(user_id);

-- Sales indexes
CREATE INDEX IF NOT EXISTS idx_customers_code ON sales.customers(customer_code);
CREATE INDEX IF NOT EXISTS idx_customers_company ON sales.customers(company_name);
CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON sales.orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_date ON sales.orders(order_date);
CREATE INDEX IF NOT EXISTS idx_orders_status ON sales.orders(status);

-- Inventory indexes
CREATE INDEX IF NOT EXISTS idx_products_code ON inventory.products(product_code);
CREATE INDEX IF NOT EXISTS idx_products_category ON inventory.products(category);
CREATE INDEX IF NOT EXISTS idx_products_active ON inventory.products(is_active);
CREATE INDEX IF NOT EXISTS idx_stock_movements_product_id ON inventory.stock_movements(product_id);
CREATE INDEX IF NOT EXISTS idx_stock_movements_date ON inventory.stock_movements(movement_date);

-- HR indexes
CREATE INDEX IF NOT EXISTS idx_employees_employee_id ON hr.employees(employee_id);
CREATE INDEX IF NOT EXISTS idx_employees_department ON hr.employees(department);
CREATE INDEX IF NOT EXISTS idx_employees_manager ON hr.employees(manager_id);
CREATE INDEX IF NOT EXISTS idx_attendance_employee_date ON hr.attendance(employee_id, attendance_date);

-- ============================================================================
-- FUNCTIONS AND TRIGGERS
-- ============================================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_posts_updated_at BEFORE UPDATE ON public.posts FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON inventory.products FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Grant permissions
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO testuser;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA sales TO testuser;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA inventory TO testuser;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA hr TO testuser;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO testuser;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA sales TO testuser;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA inventory TO testuser;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA hr TO testuser;