FROM python:3.12-alpine
ARG APP_VERSION=target
ENV APP_VERSION=${APP_VERSION}
WORKDIR /app
COPY app.py /app/app.py
CMD ["python", "/app/app.py"]
