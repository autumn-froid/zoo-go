const {request} = require('../../utils/api')

const STATUS_TEXT = {
    UNVERIFIED: '未提交',
    PENDING: '审核中',
    VERIFIED: '已通过',
    REJECTED: '未通过',
}

Page({
    data: {
        loading: true,
        submitting: false,
        verification: null,
        statusText: '未提交',
        consentContent: '',
        consentVersion: '',
        realName: '',
        idNumber: '',
        agreed: false,
        isPending: false,
        isVerified: false,
        isRejected: false,
        reviewNote: '',
    },

    onShow() {
        this.loadVerification()
    },

    async loadVerification() {
        try {
            const data = await request('/v1/identity/verification')
            const verification = data.verification || {}
            this.setData({
                loading: false,
                verification,
                statusText: STATUS_TEXT[verification.status] || '未提交',
                consentContent: data.consentContent || '',
                consentVersion: data.consentVersion || '',
                isPending: verification.status === 'PENDING',
                isVerified: verification.status === 'VERIFIED',
                isRejected: verification.status === 'REJECTED',
                reviewNote: verification.reviewNote || '',
            })
        } catch (error) {
            this.setData({loading: false})
            wx.showToast({title: error.message, icon: 'none'})
        }
    },

    onFieldInput(event) {
        const field = event.currentTarget.dataset.field
        if (field) {
            this.setData({[field]: event.detail.value})
        }
    },

    onAgreementChange(event) {
        this.setData({agreed: event.detail.value.length > 0})
    },

    async submit() {
        if (this.data.submitting) {
            return
        }
        const realName = this.data.realName.trim()
        const idNumber = this.data.idNumber.trim().toUpperCase()
        if (realName.length < 2) {
            wx.showToast({title: '请填写真实姓名', icon: 'none'})
            return
        }
        if (!/^\d{17}[\dX]$/.test(idNumber)) {
            wx.showToast({title: '请填写有效的 18 位身份证件号码', icon: 'none'})
            return
        }
        if (!this.data.agreed) {
            wx.showToast({title: '请先同意实名核验授权说明', icon: 'none'})
            return
        }
        this.setData({submitting: true})
        try {
            await request('/v1/identity/verification', {
                method: 'POST',
                data: {
                    realName,
                    idNumber,
                    consent: true,
                    consentVersion: this.data.consentVersion,
                },
            })
            wx.showToast({title: '已提交，等待核验', icon: 'success'})
            this.setData({realName: '', idNumber: '', agreed: false})
            await this.loadVerification()
        } catch (error) {
            wx.showToast({title: error.message, icon: 'none'})
        } finally {
            this.setData({submitting: false})
        }
    },
})
