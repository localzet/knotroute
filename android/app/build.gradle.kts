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
        versionCode = 30000
        versionName = providers.gradleProperty("knotVersion").orElse("3.0.0").get()
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
    implementation("androidx.webkit:webkit:1.16.0")
}
