plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.localzet.knotroute"
    compileSdk = 37

    defaultConfig {
        applicationId = "com.localzet.knotroute"
        minSdk = 26
        targetSdk = 36
        versionCode = 30000
        versionName = "3.0.0"
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

kotlin { jvmToolchain(17) }

dependencies {
    implementation(files("libs/knotroute-client.aar"))
    implementation("androidx.webkit:webkit:1.16.0")
}
