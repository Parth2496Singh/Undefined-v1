# STAGE 1: Builder
FROM python:3.12-slim AS builder

WORKDIR /app

# Install system dependencies needed to compile PostgreSQL libraries
RUN apt-get update && apt-get install -y libpq-dev gcc

# Copy requirements and compile them into Python(binaries)
COPY requirements.txt .
RUN pip wheel --no-cache-dir --no-deps --wheel-dir /app/wheels -r requirements.txt

# STAGE 2: Final Production Runner
FROM python:3.12-slim

WORKDIR /app

# Install ONLY the runtime Postgres library 
RUN apt-get update && apt-get install -y libpq5 && rm -rf /var/lib/apt/lists/*

# Copy the compiled wheels from Stage 1
COPY --from=builder /app/wheels /wheels
COPY --from=builder /app/requirements.txt .

# Install the wheels without reaching out to the internet
RUN pip install --no-cache /wheels/*

# Copy the actual Django application code
COPY . .

EXPOSE 8000

# THE ENTERPRISE FIX: Using Gunicorn instead of runserver with 3 worker processes
CMD ["sh", "-c", "python manage.py migrate && gunicorn core.wsgi:application --bind 0.0.0.0:8000 --workers 3"]