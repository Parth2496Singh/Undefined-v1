@Library("Shared") _
pipeline {
    agent { label 'vinod' } 

    environment {
        SONAR_HOME = tool "Sonar"
        // This tag is the 'trigger' for ArgoCD Image Updater
        IMAGE_TAG = "1.0.${env.BUILD_NUMBER}"
        REGISTRY_USER = "parthsingh2496"
    }
    
    stages {
        stage("Code") {
            steps {
                script {
                    clone("https://github.com/Parth2496Singh/Velzion.git", "main")
                }
            }
        }

        stage("Security Scans (Trivy & OWASP)") {
            steps {
                script {
                    trivy_scan()
                    owasp_dependency()
                }
            }
        }

        stage("SonarQube Analysis") {
            steps {
                script {
                    sonarqube_analysis("Sonar", "velzion", "velzion")
                    sonarqube_code_quality()
                }
            }
        }

        stage("Build & Push Containers") {
            steps {
                script {
                    // Backend build
                    dir('backend') {
                        build("velzion-backend", "${env.IMAGE_TAG}")
                        dockerhub_push("velzion-backend", "${env.IMAGE_TAG}", "${env.REGISTRY_USER}")
                    }
                    
                    // Frontend build with dynamic production arguments
                    // We point to the Load Balancer URL we fetched earlier
                    dir('frontend') {
                        sh """
                        docker build \\
                          --build-arg VITE_API_BASE_URL=http://54.86.145.100 \\
                          --build-arg VITE_N8N_WEBHOOK_URL=http://54.86.145.100/n8n/webhook \\
                          -t velzion-frontend:${env.IMAGE_TAG} . 
                        """
                        dockerhub_push("velzion-frontend", "${env.IMAGE_TAG}", "${env.REGISTRY_USER}")
                    }
                }
            }
        }
    }
}
