FROM python:3.12-alpine
ARG BROKEN=1
ENV BROKEN=${BROKEN}
WORKDIR /app
COPY app.py /app/app.py
CMD ["python", "/app/app.py"]
