plugins {
    id("com.android.application")
}

android {
    namespace = "com.localzet.knotroute"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.localzet.knotroute"
        minSdk = 26
        targetSdk = 36
        versionCode = 40001
        versionName = providers.gradleProperty("knotVersion").orElse("4.0.0-alpha.1").get()
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

dependencies {
    implementation(files("libs/knotroute-client.aar"))
    implementation("com.google.android.material:material:1.14.0")
}
